package extcmd

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"sync"
)

// Run implements a closure using an external command, operating in a
// ping-pong fashion, e.g. to convert timestamps to dates, for testing.
// Arbitrary input and output formats are supported, via the various functions.
// The appendInput may implement arbitrary behavior, e.g. it might append a
// trailing newline as a end-of-input delimiter. The provided [bufio.SplitFunc]
// will be used to split the output from the command, as an output delimiter.
// This output will then be parsed by the parseOutput function.
// The f function will be called with the context and a function that can be
// used to send input to the command, and receive output from the command.
func Run[
	// to closure
	Input any,
	// from closure
	Output any,
](
	ctx context.Context,
	command string,
	args []string,
	appendInput func(b []byte, input Input) ([]byte, error),
	splitOutput bufio.SplitFunc,
	parseOutput func(b []byte) (Output, error),
	f func(ctx context.Context, call func(input Input) (Output, error)) error,
) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	rIn, wIn := io.Pipe()
	defer rIn.Close()
	rOut, wOut := io.Pipe()
	defer rOut.Close()

	inputs := make(chan []byte)
	go func() {
		defer func() { _ = wIn.CloseWithError(context.Cause(ctx)) }()
		for ctx.Err() == nil {
			select {
			case <-ctx.Done():
				return
			case v := <-inputs:
				if _, err := wIn.Write(v); err != nil {
					cancel(err)
					return
				}
			}
		}
	}()

	outputs := make(chan Output)
	go func() {
		defer rOut.Close()
		r := bufio.NewScanner(rOut)
		r.Split(splitOutput)
		for r.Scan() {
			v, err := parseOutput(r.Bytes())
			if err != nil {
				cancel(err)
				return
			}
			select {
			case <-ctx.Done():
				return
			case outputs <- v:
			}
		}
		if err := r.Err(); err != nil {
			cancel(err)
		}
	}()

	c := exec.CommandContext(ctx, command, args...)
	c.Stderr = os.Stderr
	c.Stdin = rIn
	c.Stdout = wOut

	go func() {
		defer rIn.Close()
		defer wOut.Close()
		var err error
		defer func() {
			cancel(err)
			_ = wOut.CloseWithError(context.Cause(ctx))
		}()
		err = c.Run()
	}()

	var (
		mu  sync.Mutex
		buf []byte
	)

	call := func(input Input) (output Output, err error) {
		mu.Lock()
		defer mu.Unlock()

		if ctx.Err() != nil {
			return output, context.Cause(ctx)
		}

		buf = buf[:0]

		buf, err = appendInput(buf, input)
		if err != nil {
			return output, err
		}

		select {
		case <-ctx.Done():
			return output, context.Cause(ctx)
		case inputs <- buf:
		}

		select {
		case <-ctx.Done():
			return output, context.Cause(ctx)
		case output = <-outputs:
			return output, nil
		}
	}

	return f(ctx, call)
}
