package main

import (
	"fmt"
	"sync"

	"github.com/mariomac/pipes/pipe"
)

func ranger(from, to int) func(out chan<- int) {
	return func(out chan<- int) {
		for i := from; i < to; i++ {
			out <- i
		}
	}
}

func evenFilter(in <-chan int, out chan<- int) {
	for n := range in {
		if n%2 != 0 {
			out <- n
		}
	}
}

func multiplier(factor int) func(in <-chan int, out chan<- int) {
	return func(in <-chan int, out chan<- int) {
		for n := range in {
			out <- n * factor
		}
	}
}

func printer(in <-chan int) {
	for n := range in {
		fmt.Print(n, " ")
	}
	fmt.Println()
}

func channelSpread[T any](in <-chan T, outs ...chan<- T) {
	for n := range in {
		for _, out := range outs {
			out <- n
		}
	}
}

func runManualPipeline() {
	ch1 := make(chan int)
	ch1_1 := make(chan int)
	ch1_2 := make(chan int)
	ch2 := make(chan int)

	go func() {
		ranger(5, 15)(ch1)
		close(ch1)
	}()
	go func() {
		channelSpread(ch1, ch1_1, ch1_2)
		close(ch1_1)
		close(ch1_2)
	}()
	closeCh2 := sync.WaitGroup{}
	closeCh2.Add(2)
	go func() {
		evenFilter(ch1_1, ch2)
		closeCh2.Done()
	}()
	go func() {
		multiplier(3)(ch1_2, ch2)
		closeCh2.Done()
	}()
	go func() {
		closeCh2.Wait()
		close(ch2)
	}()
	printer(ch2)
}

type Pipeline struct {
	ranger     pipe.Start[int]
	filter     pipe.Middle[int, int]
	multiplier pipe.Middle[int, int]
	printer    pipe.Final[int]
}

func (p *Pipeline) Ranger() *pipe.Start[int]           { return &p.ranger }
func (p *Pipeline) Filter() *pipe.Middle[int, int]     { return &p.filter }
func (p *Pipeline) Multiplier() *pipe.Middle[int, int] { return &p.multiplier }
func (p *Pipeline) Printer() *pipe.Final[int]          { return &p.printer }

func (p *Pipeline) Connect() {
	p.ranger.SendTo(p.filter, p.multiplier)
	p.multiplier.SendTo(p.printer)
	p.filter.SendTo(p.printer)
}

func runAutoPipe() {
	// builder and register nodes
	builder := pipe.NewBuilder(&Pipeline{}, pipe.ChannelBufferLen(1))
	pipe.AddStart(builder, (*Pipeline).Ranger, ranger(5, 15))
	pipe.AddMiddle(builder, (*Pipeline).Filter, evenFilter)
	pipe.AddMiddle(builder, (*Pipeline).Multiplier, multiplier(3))
	pipe.AddFinal(builder, (*Pipeline).Printer, printer)
	run, _ := builder.Build()
	run.Start()
	<-run.Done()
}

func main() {
	fmt.Print("Manual: ")
	runManualPipeline()
	fmt.Print("Auto:   ")
	runAutoPipe()
}
