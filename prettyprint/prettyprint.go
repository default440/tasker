package prettyprint

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/hokaccha/go-prettyjson"
)

func JSONObjectColor(obj any) {
	s, _ := prettyjson.Marshal(obj)
	fmt.Println(string(s))
}

func JSONObject(obj any) {
	j, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(j))
}

func JSONObjectToColorString(obj any) string {
	s, _ := prettyjson.Marshal(obj)
	return string(s)
}
