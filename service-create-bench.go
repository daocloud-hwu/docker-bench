package main

import (
    "fmt"
    "os"
    "sync"
    "time"

    "golang.org/x/net/context"
    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/swarm"
    "github.com/docker/docker/client"
    "github.com/codegangsta/cli"
    "github.com/montanaflynn/stats"
)

const MILLIS_IN_SECOND = 1000

func worker(requests int, image string, args []string, completeCh chan time.Duration) {
    var service swarm.ServiceSpec
    var options types.ServiceCreateOptions
    service.TaskTemplate.ContainerSpec.Image = image
    service.TaskTemplate.ContainerSpec.Args = args

    for i := 0; i < requests; i++ {
        start := time.Now()
        c, err := client.NewClient("unix:///var/run/docker.sock", "v1.24", nil, map[string]string{"User-Agent": "engine-api-cli-1.0"})
        if err != nil {
            panic(err)
        }

        if _, err := c.ServiceCreate(context.Background(), service, options); err != nil {
            panic(err)
        }
        completeCh <- time.Since(start)
    }
}

func session(requests, concurrency int, images []string, args []string, completeCh chan time.Duration) {
    var wg sync.WaitGroup
    var size = len(images)
    n := requests / concurrency

    for i := 0; i < concurrency; i++ {
        wg.Add(1)
        image := images[i%size]
        go func() {
            worker(n, image, args, completeCh)
            wg.Done()
        }()
    }
    wg.Wait()
}

func bench(requests, concurrency int, images []string, args []string) {
    start := time.Now()

    timings := make([]float64, 0)
    completeCh := make(chan time.Duration, requests)
    doneCh := make(chan struct{})
    current := 0
    go func() {
        for timing := range completeCh {
            timings = append(timings, timing.Seconds())
            current++
            percent := float64(current) / float64(requests) * 100
            fmt.Printf("[%3.f%%] %d/%d services started\n", percent, current, requests)
        }
        doneCh <- struct{}{}
    }()
    session(requests, concurrency, images, args, completeCh)
    close(completeCh)
    <-doneCh

    total := time.Since(start)
    mean, _ := stats.Mean(timings)
    p90th, _ := stats.Percentile(timings, 90)
    p99th, _ := stats.Percentile(timings, 99)

    meanMillis := mean * MILLIS_IN_SECOND
    p90thMillis := p90th * MILLIS_IN_SECOND
    p99thMillis := p99th * MILLIS_IN_SECOND

    fmt.Printf("\n")
    fmt.Printf("Time taken for tests: %.3fs\n", total.Seconds())
    fmt.Printf("Time per service: %.3fms [mean] | %.3fms [90th] | %.3fms [99th]\n", meanMillis, p90thMillis, p99thMillis)
}

func main() {
    app := cli.NewApp()
    app.Name = "service-create-bench"
    app.Usage = "DaoCloud swarm-kit benchmarking tool"
    app.Version = "0.1"
    app.Author = "haipeng"
    app.Email = "haipeng.wu@daocloud.io"
    app.Flags = []cli.Flag{
        cli.IntFlag{
            Name:  "concurrency, c",
            Value: 1,
            Usage: "Number of multiple requests to perform at a time.",
        },
        cli.IntFlag{
            Name:  "requests, r",
            Value: 1,
            Usage: "Number of services to start for the benchmarking session.",
        },
        cli.StringSliceFlag{
            Name:  "image, i",
            Value: &cli.StringSlice{},
            Usage: "Image(s) to use for benchmarking.",
        },
    }

    app.Action = func(c *cli.Context) {
        if !c.IsSet("image") && !c.IsSet("i") {
            cli.ShowAppHelp(c)
            os.Exit(1)
        }
        bench(c.Int("requests"), c.Int("concurrency"), c.StringSlice("image"), c.Args())
    }

    app.Run(os.Args)
}
