package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/mariomac/pipes/pipe"
)

const numSamples = 10

type Measure struct {
	Timestamp time.Time
	Value     float64
}

// forwards fake/random measures every 100 milliseconds
func measurer(numSamples int) func(out chan<- Measure) {
	return func(out chan<- Measure) {
		clock := time.NewTicker(100 * time.Millisecond)
		for i := 0; i < numSamples; i++ {
			out <- Measure{<-clock.C, rand.Float64()}
		}
	}
}

// sums the values of the measures within the same second
// and forwards them as a single added measure for that second
func summarizer(in <-chan Measure, out chan<- Measure) {
	currentSeconds := time.Now().Unix()
	sum := float64(0)
	for measure := range in {
		if measure.Timestamp.Unix() != currentSeconds {
			out <- Measure{time.Unix(currentSeconds, 0), sum}
			currentSeconds++
			sum = 0
		}
		sum += measure.Value
	}
	out <- Measure{time.Unix(currentSeconds, 0), sum}
}

func logger(in <-chan Measure) {
	for measure := range in {
		fmt.Println(measure.Timestamp, "->", measure.Value)
	}
}

func runManualPipeline() {
	ch1 := make(chan Measure)
	ch2 := make(chan Measure)
	go func() {
		measurer(numSamples)(ch1)
		close(ch1)
	}()
	go func() {
		summarizer(ch1, ch2)
		close(ch2)
	}()
	logger(ch2)
}

type Pipeline struct {
	measurer   pipe.Start[Measure]
	summarizer pipe.Middle[Measure, Measure]
	logger     pipe.Final[Measure]
}

func (p *Pipeline) Ranger() *pipe.Start[Measure]           { return &p.measurer }
func (p *Pipeline) Filter() *pipe.Middle[Measure, Measure] { return &p.summarizer }
func (p *Pipeline) Printer() *pipe.Final[Measure]          { return &p.logger }

func (p *Pipeline) Connect() {
	p.measurer.SendTo(p.summarizer)
	p.summarizer.SendTo(p.logger)
}

func runAutoPipe() {
	// builder and register nodes
	builder := pipe.NewBuilder(&Pipeline{})
	pipe.AddStart(builder, (*Pipeline).Ranger, measurer(numSamples))
	pipe.AddMiddle(builder, (*Pipeline).Filter, summarizer)
	pipe.AddFinal(builder, (*Pipeline).Printer, logger)
	run, _ := builder.Build()
	run.Start()
	<-run.Done()
}

func main() {
	fmt.Println("=== Manual ===")
	runManualPipeline()
	fmt.Println("=== Auto ===")
	runAutoPipe()
}
