package prettyprint

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/hokaccha/go-prettyjson"
)

func JSONObjectColor(obj interface{}) {
	s, _ := prettyjson.Marshal(obj)
	fmt.Println(string(s))
}

func JSONObject(obj interface{}) {
	j, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(j))
}
