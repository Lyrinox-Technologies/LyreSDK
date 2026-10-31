// Identity object administration over a standard human Lyre sign-in.
// Supply LYRE_USERNAME and LYRE_PASSWORD through a private process environment.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	sdk "github.com/Lyrinox-Technologies/LyreSDK"
	"log"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	address := flag.String("url", "wss://lyre.lyrinox.com/ws", "Lyre endpoint")
	service := flag.String("service", "syscodex-provider", "object service scope")
	action := flag.String("action", "list", "list, get, revoke or restore")
	kind := flag.String("kind", "server", "object kind filter")
	id := flag.String("id", "", "object ID")
	after := flag.String("after", "", "list cursor")
	flag.Parse()
	switch *action {
	case "list", "get", "revoke", "restore":
	default:
		log.Fatal("unsupported administration action")
	}
	u, e := url.Parse(*address)
	if e != nil || u.Scheme != "wss" || u.Host == "" || u.User != nil {
		log.Fatal("verified WSS required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	c, e := sdk.Dial(ctx, *address, sdk.Config{Timeout: 5 * time.Minute})
	if e != nil {
		log.Fatal("connection failed")
	}
	defer c.Close()
	c.OnMFARequired(func(kind, hint string) string {
		fmt.Fprintf(os.Stderr, "Verification (%s, %s): ", kind, hint)
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		return strings.TrimSpace(line)
	})
	password := os.Getenv("LYRE_PASSWORD")
	os.Unsetenv("LYRE_PASSWORD")
	if e = c.Login(os.Getenv("LYRE_USERNAME"), password); e != nil {
		log.Fatal("sign-in failed")
	}
	password = ""
	data, e := c.IdentityObjects(ctx, sdk.IdentityObjectRequest{Action: *action, Service: *service, Kind: *kind, ID: *id, After: *after})
	if e != nil {
		log.Fatal(e)
	}
	encoded, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(encoded))
}
