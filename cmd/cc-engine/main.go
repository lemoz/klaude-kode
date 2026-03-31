package main

import (
	"fmt"

	"github.com/cdossman/klaude-kode/internal/engine"
)

func main() {
	e := engine.NewInMemoryEngine()
	fmt.Printf("Klaude Kode engine bootstrap\n")
	fmt.Printf("engine implementation: %T\n", e)
	fmt.Printf("status: blueprint scaffold only\n")
}

