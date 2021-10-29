package prettyprint

import (
	"encoding/json"
	"log"
)

func JSONObject(obj interface{}) {
	j, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(string(j))
}
