package main

import "github.com/informalsystems/conformance-tests/generator"

func main() {
	generator.GenerateManyHeaderBisectionCases("./tests/json/val_list.json")
	generator.GenerateSingleStepSkippingCases("./tests/json/val_list.json")
	generator.GenerateSingleStepSequentialCases("./tests/json/val_list.json")
}
