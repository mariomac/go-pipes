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

func averager(in <-chan Measure, out chan<- Measure) {
	currentSeconds := time.Now().Unix()
	var sum, count float64 = 0, 0
	for measure := range in {
		if measure.Timestamp.Unix() != currentSeconds {
			if count != 0 {
				out <- Measure{time.Unix(currentSeconds, 0), sum / count}
			}
			currentSeconds++
			sum, count = 0, 0
		}
		sum += measure.Value
		count++
	}
	if count != 0 {
		out <- Measure{time.Unix(currentSeconds, 0), sum / count}
	}
}

func logger(in <-chan Measure) {
	for measure := range in {
		fmt.Println(measure.Timestamp, "->", measure.Value)
	}
}

func runManualPipeline() {
	measureOut := make(chan Measure)
	averageOut := make(chan Measure)
	go func() {
		measurer(numSamples)(measureOut)
		close(measureOut)
	}()
	go func() {
		averager(measureOut, averageOut)
		close(averageOut)
	}()
	logger(averageOut)
}

type Pipeline struct {
	measurer pipe.Start[Measure]
	averager pipe.Middle[Measure, Measure]
	logger   pipe.Final[Measure]
}

func (p *Pipeline) Measurer() *pipe.Start[Measure]           { return &p.measurer }
func (p *Pipeline) Averager() *pipe.Middle[Measure, Measure] { return &p.averager }
func (p *Pipeline) Logger() *pipe.Final[Measure]             { return &p.logger }

func (p *Pipeline) Connect() {
	p.measurer.SendTo(p.averager)
	p.averager.SendTo(p.logger)
}

func runAutoPipe() {
	// builder and register nodes
	builder := pipe.NewBuilder(&Pipeline{})
	pipe.AddStart(builder, (*Pipeline).Measurer, measurer(numSamples))
	pipe.AddMiddle(builder, (*Pipeline).Averager, averager)
	pipe.AddFinal(builder, (*Pipeline).Logger, logger)
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
