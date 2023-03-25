package main

import (
	"fmt"
	"os"
	"testing"
)

func TestXxx(t *testing.T) {
	fmt.Println(os.ExpandEnv("$GOPATH/src/woongkie-talkie/.env"))
}
