package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

func main() {
	data, _ := ioutil.ReadFile("users.txt")
	users := string(data)

	for _, name := range split(users) {
		resp, _ := http.Get("https://api.example.com/users/" + name)
		body, _ := ioutil.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)
		fmt.Println(result)
	}
}

func split(s string) []string {
	var out []string
	word := ""
	for _, c := range s {
		if c == '\n' {
			out = append(out, word)
			word = ""
		} else {
			word += string(c)
		}
	}
	return out
}