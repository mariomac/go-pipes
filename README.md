# PIPES: Processing In Pipeline-Embedded Stages

PIPES is a library that allows to dynamically connect multiple pipeline
stages that are communicated via channels. Each stage will run in a goroutine.

API doc: https://pkg.go.dev/github.com/mariomac/pipes

It is the upper-upstream fork version of the [Red Hat's & IBM Gopipes library](https://pkg.go.dev/github.com/netobserv/gopipes)
and the core parts of the [Red Hat's & IBM Flowlogs pipeline](https://github.com/netobserv/flowlogs-pipeline),
where I plan to add experimental features that aren't related to any concrete product nor follow
any peer review nor company standard.

This library allows wrapping functions within Nodes of a graph. In order to pass data across
the nodes, each wrapped function must receive, as arguments, an input channel, an output channel,
or both.

It has two usable API layers: the **node** API (low-level), where you manually instantiate and wire every
node; and the **graph** API (high-level), that allows you providing a predefined set of nodes that are
automatically wired via configuration file.

## Node low-level API

There are three types of nodes:

* **Starting** node: each of the starting point of a graph. This is, all the nodes that bring information
  from outside the graph: e.g. because they generate them or because they acquire them from an
  external source like a Web Service. A graph must have at least one Start node. A Start node must 
  have at least one output node.
* **Middle** node: any intermediate node that receives data from another node, processes/filters it,
  and forwards the data to another node. A Middle node must have at least one output node.
* **Terminal** node: any node that receives data from another node and does not forward it to
  another node, but can process it and send the results to outside the graph
  (e.g. memory, storage, web...)

## Example pipeline for the node API

The following pipeline has two Start nodes that send the data to two destination Middle
nodes (`odds` and `evens`). From there, the data follows their own branches until they
are eventually joined in the `printer` Terminal node.

Check the complete examples in the [examples/](./examples) folder).

```go
func main() {
	// Defining Start, middle and terminal nodes that wrap some functions
	start1 := node.AsStart(StartCounter)
	start2 := node.AsStart(StartRandoms)
	odds := node.AsMiddle(OddFilter)
	evens := node.AsMiddle(EvenFilter)
	oddsMsg := node.AsMiddle(Messager("odd number"))
	evensMsg := node.AsMiddle(Messager("even number"))
	printer := node.AsTerminal(Printer)
	
	// Connecting nodes like:
	//
    // start1----\ /---start2
    //   |        X      |
    //  evens<---/ \-->odds
    //   |              |
    //  evensMsg      oddsMsg
    //        \       /
    //         printer

	start1.SendsTo(evens, odds)
	start2.SendsTo(evens, odds)
	odds.SendsTo(oddsMsg)
	evens.SendsTo(evensMsg)
	oddsMsg.SendsTo(printer)
	evensMsg.SendsTo(printer)

	// all the Start nodes must be started to
	// start forwarding data to the rest of the graph
	start1.Start()
	start2.Start()

    // We can wait for terminal nodes to finish their execution
    // after the rest of the graph has finished
    <-printer.Done()
}
```

Output:

```
even number: 2
odd number: 847
odd number: 59
odd number: 81
odd number: 81
even number: 0
odd number: 3
odd number: 1
odd number: 887
even number: 4
```

## Graph high-level API

TBD