package prettyprint

import (
	"encoding/json"
	"fmt"
	"log"
)

func JSONObject(obj interface{}) {
	j, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(j))
}
