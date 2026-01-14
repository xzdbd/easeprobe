package main

import (
	"encoding/json"
	"fmt"

	"github.com/megaease/easeprobe/conf"
)

func main() {
	jsonStr := `{"http": [{"name": "test", "url": "http://example.com"}]}`
	var c conf.Conf
	err := json.Unmarshal([]byte(jsonStr), &c)
	if err != nil {
		fmt.Println("Error:", err)
	}
	fmt.Printf("%+v\n", c)
}
