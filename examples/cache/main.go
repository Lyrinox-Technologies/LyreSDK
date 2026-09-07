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
	cache := c.Cache("example_"+ids.UUIDs[0], lyresdk.CacheOptions{ProviderID: "lyrinox"})
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := cache.Clear(cleanup); err != nil {
			log.Printf("cache cleanup failed; TTL remains: %v", err)
		}
	}()
	if _, err = cache.Put(ctx, "message", "Hello from LyreSDK", time.Minute); err != nil {
		return err
	}
	var message string
	_, found, err := cache.Get(ctx, "message", &message)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("cache entry expired or was evicted")
	}
	fmt.Println(message)
	return nil
}
