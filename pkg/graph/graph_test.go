package graph

import (
	"context"
	"testing"
	"time"

	"github.com/mariomac/pipes/pkg/graph/stage"
	"github.com/mariomac/pipes/pkg/node"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
	b := NewBuilder()

	type CounterCfg struct {
		stage.Instance
		From int
		To   int
	}
	RegisterStart(b, func(cfg CounterCfg) node.StartFuncCtx[int] {
		return func(_ context.Context, out chan<- int) {
			for i := cfg.From; i <= cfg.To; i++ {
				out <- i
			}
		}
	})

	type DoublerCfg struct {
		stage.Instance
	}
	RegisterMiddle(b, func(_ DoublerCfg) node.MiddleFunc[int, int] {
		return func(in <-chan int, out chan<- int) {
			for n := range in {
				out <- n * 2
			}
		}
	})

	type MapperCfg struct {
		stage.Instance
		Dst map[int]struct{}
	}
	RegisterTerminal(b, func(cfg MapperCfg) node.TerminalFunc[int] {
		return func(in <-chan int) {
			for n := range in {
				cfg.Dst[n] = struct{}{}
			}
		}
	})

	type config struct {
		Starts []CounterCfg
		Middle DoublerCfg
		Term   []MapperCfg
		Connector
	}
	map1, map2 := map[int]struct{}{}, map[int]struct{}{}
	g, err := b.Build(config{
		Starts: []CounterCfg{
			{From: 1, To: 5, Instance: "c1"},
			{From: 6, To: 8, Instance: "c2"}},
		Middle: DoublerCfg{Instance: "d"},
		Term: []MapperCfg{
			{Dst: map1, Instance: "m1"},
			{Dst: map2, Instance: "m2"}},
		Connector: Connector{
			"c1": {"d"},
			"c2": {"d"},
			"d":  {"m1", "m2"},
		},
	})
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		g.Run(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.Fail(t, "timeout while waiting for graph to complete")
	}

	assert.Equal(t, map[int]struct{}{2: {}, 4: {}, 6: {}, 8: {}, 10: {}, 12: {}, 14: {}, 16: {}}, map1)
	assert.Equal(t, map1, map2)
}

func TestContext(t *testing.T) {
	b := NewBuilder()

	type ReceiverCfg struct {
		stage.Instance
		Input chan int
	}
	RegisterStart(b, func(cfg ReceiverCfg) node.StartFuncCtx[int] {
		return func(ctx context.Context, out chan<- int) {
			for {
				select {
				case <-ctx.Done():
					return
				case i := <-cfg.Input:
					out <- i
				}
			}
		}
	})

	type ForwarderCfg struct {
		stage.Instance
		Out chan int
	}
	allClosed := make(chan struct{})
	RegisterTerminal(b, func(cfg ForwarderCfg) node.TerminalFunc[int] {
		return func(in <-chan int) {
			for n := range in {
				cfg.Out <- n
			}
			close(allClosed)
		}
	})
	type config struct {
		Starts []ReceiverCfg
		Term   ForwarderCfg
		Connector
	}
	cfg := config{
		Starts: []ReceiverCfg{
			{Instance: "start1", Input: make(chan int, 10)},
			{Instance: "start2", Input: make(chan int, 10)},
		},
		Term: ForwarderCfg{Instance: "end", Out: make(chan int)},
		Connector: Connector{
			"start1": []string{"end"},
			"start2": []string{"end"},
		},
	}
	g, err := b.Build(&cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	go g.Run(ctx)

	// The graph works normally
	cfg.Starts[0].Input <- 123
	select {
	case o := <-cfg.Term.Out:
		assert.Equal(t, 123, o)
	case <-time.After(timeout):
		assert.Fail(t, "timeout while waiting for graph to forward items")
	}
	cfg.Starts[1].Input <- 456
	select {
	case o := <-cfg.Term.Out:
		assert.Equal(t, 456, o)
	case <-time.After(timeout):
		assert.Fail(t, "timeout while waiting for graph to forward items")
	}

	// after canceling context, the graph should not forward anything
	cancel()

	cfg.Starts[0].Input <- 789
	cfg.Starts[1].Input <- 101
	select {
	case o := <-cfg.Term.Out:
		assert.Failf(t, "graph should have been stopped", "unexpected output of the graph: %v", o)
	default:
		// OK!
	}

}

func TestNodeIdAsTag(t *testing.T) {
	b := NewBuilder()

	type CounterCfg struct {
		From int
		To   int
	}
	RegisterStart(b, func(cfg CounterCfg) node.StartFuncCtx[int] {
		return func(_ context.Context, out chan<- int) {
			for i := cfg.From; i <= cfg.To; i++ {
				out <- i
			}
		}
	})

	type DoublerCfg struct{}
	RegisterMiddle(b, func(_ DoublerCfg) node.MiddleFunc[int, int] {
		return func(in <-chan int, out chan<- int) {
			for n := range in {
				out <- n * 2
			}
		}
	})

	type MapperCfg struct {
		Dst map[int]struct{}
	}
	RegisterTerminal(b, func(cfg MapperCfg) node.TerminalFunc[int] {
		return func(in <-chan int) {
			for n := range in {
				cfg.Dst[n] = struct{}{}
			}
		}
	})

	type config struct {
		Starts CounterCfg `nodeId:"s"`
		Middle DoublerCfg `nodeId:"m"`
		Term   MapperCfg  `nodeId:"t"`
		Connector
	}
	map1 := map[int]struct{}{}
	g, err := b.Build(config{
		Starts: CounterCfg{From: 1, To: 5},
		Middle: DoublerCfg{},
		Term:   MapperCfg{Dst: map1},
		Connector: Connector{
			"s": {"m"},
			"m": {"t"},
		},
	})
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		g.Run(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.Fail(t, "timeout while waiting for graph to complete")
	}

	assert.Equal(t, map[int]struct{}{2: {}, 4: {}, 6: {}, 8: {}, 10: {}}, map1)
}

func TestSendsTo(t *testing.T) {
	b := NewBuilder()

	type CounterCfg struct {
		From int
		To   int
	}
	RegisterStart(b, func(cfg CounterCfg) node.StartFuncCtx[int] {
		return func(_ context.Context, out chan<- int) {
			for i := cfg.From; i <= cfg.To; i++ {
				out <- i
			}
		}
	})

	type DoublerCfg struct{}
	RegisterMiddle(b, func(_ DoublerCfg) node.MiddleFunc[int, int] {
		return func(in <-chan int, out chan<- int) {
			for n := range in {
				out <- n * 2
			}
		}
	})

	type MapperCfg struct {
		Dst map[int]struct{}
	}
	RegisterTerminal(b, func(cfg MapperCfg) node.TerminalFunc[int] {
		return func(in <-chan int) {
			for n := range in {
				cfg.Dst[n] = struct{}{}
			}
		}
	})

	type config struct {
		Starts CounterCfg `nodeId:"s" sendsTo:"m"`
		Middle DoublerCfg `nodeId:"m" sendsTo:"t"`
		Term   MapperCfg  `nodeId:"t"`
	}
	map1 := map[int]struct{}{}
	g, err := b.Build(config{
		Starts: CounterCfg{From: 1, To: 5},
		Middle: DoublerCfg{},
		Term:   MapperCfg{Dst: map1},
	})
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		g.Run(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		require.Fail(t, "timeout while waiting for graph to complete")
	}

	assert.Equal(t, map[int]struct{}{2: {}, 4: {}, 6: {}, 8: {}, 10: {}}, map1)
}

func TestSendsTo_WrongAnnotations(t *testing.T) {
	b := NewBuilder()

	type CounterCfg struct {
		From int
		To   int
	}
	RegisterStart(b, func(cfg CounterCfg) node.StartFuncCtx[int] {
		return func(_ context.Context, out chan<- int) {}
	})

	type DoublerCfg struct{}
	RegisterMiddle(b, func(_ DoublerCfg) node.MiddleFunc[int, int] {
		return func(in <-chan int, out chan<- int) {}
	})

	type MapperCfg struct {
		Dst map[int]struct{}
	}
	RegisterTerminal(b, func(cfg MapperCfg) node.TerminalFunc[int] {
		return func(in <-chan int) {}
	})

	// Should fail because a node is missing a sendsTo
	type config1 struct {
		Starts CounterCfg `nodeId:"s"`
		Middle DoublerCfg `nodeId:"m" sendsTo:"t"`
		Term   MapperCfg  `nodeId:"t"`
	}
	_, err := b.Build(config1{})
	assert.Error(t, err)

	// Should fail because the middle node is missing a sendsTo
	type config2 struct {
		Starts CounterCfg `nodeId:"s" sendsTo:"m"`
		Middle DoublerCfg `nodeId:"m"`
		Term   MapperCfg  `nodeId:"t"`
	}
	_, err = b.Build(config2{})
	assert.Error(t, err)

	// Should fail because the middle node is sending to a start node
	type config3 struct {
		Starts CounterCfg `nodeId:"s"`
		Middle DoublerCfg `nodeId:"m"  sendsTo:"s"`
		Term   MapperCfg  `nodeId:"t"`
	}
	_, err = b.Build(config3{})
	assert.Error(t, err)

	// Should fail because a node cannot send to itself
	type config4 struct {
		Starts CounterCfg `nodeId:"s"  sendsTo:"m,t"`
		Middle DoublerCfg `nodeId:"m"  sendsTo:"m"`
		Term   MapperCfg  `nodeId:"t"`
	}
	_, err = b.Build(config4{})
	assert.Error(t, err)

	// Should fail because a destination node does not exist
	type config5 struct {
		Starts CounterCfg `nodeId:"s" sendsTo:"m,x"`
		Middle DoublerCfg `nodeId:"m" sendsTo:"t"`
		Term   MapperCfg  `nodeId:"t"`
	}
	_, err = b.Build(config5{})
	assert.Error(t, err)

	// Should fail because the middle node does not have any input
	type config6 struct {
		Starts CounterCfg `nodeId:"s" sendsTo:"t"`
		Middle DoublerCfg `nodeId:"m" sendsTo:"t"`
		Term   MapperCfg  `nodeId:"t"`
	}
	_, err = b.Build(config6{})
	assert.Error(t, err)
}
