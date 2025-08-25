package main

import "github.com/codecflow/fabric/stash/config"

func main() {
	cm, err := config.NewManager[byte]()
	if err != nil {
		panic(err)
	}

	print(cm.Print())
}
