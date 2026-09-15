package main

import (
	"context"
	"fmt"
	lyresdk "github.com/Lyrinox-Technologies/LyreSDK"
	"log"
	"os"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, err := lyresdk.Dial(ctx, os.Getenv("LYRE_URL"), lyresdk.Config{})
	if err != nil {
		return err
	}
	defer c.Close()
	if err = c.LoginAgent(ctx, os.Getenv("LYRE_AGENT_KEY")); err != nil {
		return err
	}
	var ids struct {
		UUIDs []string `json:"uuids"`
	}
	if err = c.Call(ctx, "lyre.uuid.generate@v1", map[string]any{"version": "v4", "count": 1}, &ids); err != nil {
		return err
	}
	if len(ids.UUIDs) != 1 {
		return fmt.Errorf("provider returned no UUID")
	}
	namespace := "example_" + ids.UUIDs[0]
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var out struct {
			Deleted int `json:"deleted"`
		}
		if err := c.Call(cleanup, "lyre.cache.clear@v1:lyrinox.cache.clear", map[string]any{"namespace": namespace}, &out); err != nil {
			log.Printf("cache cleanup failed; TTL remains: %v", err)
		}
	}()
	var put struct {
		Revision string `json:"revision"`
	}
	if err = c.Call(ctx, "lyre.cache.put@v1:lyrinox.cache.put", map[string]any{"namespace": namespace, "key": "message", "value": "Hello from LyreSDK", "ttl_seconds": 60}, &put); err != nil {
		return err
	}
	var got struct {
		Found bool   `json:"found"`
		Value string `json:"value"`
	}
	if err = c.Call(ctx, "lyre.cache.get@v1:lyrinox.cache.get", map[string]any{"namespace": namespace, "key": "message"}, &got); err != nil {
		return err
	}
	if !got.Found {
		return fmt.Errorf("cache entry expired or was evicted")
	}
	fmt.Println(got.Value)
	return nil
}
