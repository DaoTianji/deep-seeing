// Run explicitly to prepare a private, allowlisted systemd environment file.
package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		panic("usage: prepare-reading-config source.env new-output.env")
	}
	values, err := godotenv.Read(os.Args[1])
	if err != nil {
		panic("cannot read source configuration")
	}
	var out strings.Builder
	for _, key := range []string{"OPENAI_API_KEY", "OPENAI_BASE_URL", "OPENAI_MODEL"} {
		v := values[key]
		if v == "" || strings.ContainsAny(v, "\r\n\x00") {
			panic("missing or invalid " + key)
		}
		out.WriteString(key + "=" + strconv.Quote(v) + "\n")
	}
	out.WriteString("STORY_MODE=agent\nSTORY_PROVIDER=gateway\n")
	f, err := os.OpenFile(os.Args[2], os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		panic("cannot create private destination")
	}
	defer f.Close()
	if _, err = f.WriteString(out.String()); err != nil {
		panic("cannot save configuration")
	}
	fmt.Println("Prepared model-only configuration; no database, identity or private-memory settings copied.")
}
