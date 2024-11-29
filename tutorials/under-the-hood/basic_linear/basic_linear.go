package main

import (
	"fmt"

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

func printer(in <-chan int) {
	for n := range in {
		fmt.Print(n, " ")
	}
	fmt.Println()
}

func runManualPipeline() {
	ch1 := make(chan int)
	ch2 := make(chan int)
	go func() {
		ranger(5, 15)(ch1)
		close(ch1)
	}()
	go func() {
		evenFilter(ch1, ch2)
		close(ch2)
	}()
	printer(ch2)
}

type Pipeline struct {
	ranger  pipe.Start[int]
	filter  pipe.Middle[int, int]
	printer pipe.Final[int]
}

func (p *Pipeline) Ranger() *pipe.Start[int]       { return &p.ranger }
func (p *Pipeline) Filter() *pipe.Middle[int, int] { return &p.filter }
func (p *Pipeline) Printer() *pipe.Final[int]      { return &p.printer }

func (p *Pipeline) Connect() {
	p.ranger.SendTo(p.filter)
	p.filter.SendTo(p.printer)
}

func runAutoPipe() {
	// builder and register nodes
	builder := pipe.NewBuilder(&Pipeline{}, pipe.ChannelBufferLen(1))
	pipe.AddStart(builder, (*Pipeline).Ranger, ranger(5, 15))
	pipe.AddMiddle(builder, (*Pipeline).Filter, evenFilter)
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
