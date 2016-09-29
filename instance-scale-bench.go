package main

import (
    "fmt"
    "os"
    "time"

    "golang.org/x/net/context"
    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/swarm"
    "github.com/docker/docker/client"
    "github.com/codegangsta/cli"
    "github.com/montanaflynn/stats"
)

func newClient() *client.Client {
    c, err := client.NewClient("unix:///var/run/docker.sock", "v1.24", nil, map[string]string{"User-Agent": "engine-api-cli-1.0"})
    if err != nil {
        panic(err)
    }

    return c
}

func scale(n uint64, image string, args []string) float64 {
    var service swarm.ServiceSpec
    var options types.ServiceCreateOptions
    service.TaskTemplate.ContainerSpec.Image = image
    service.TaskTemplate.ContainerSpec.Args = args
    service.Mode.Replicated = &swarm.ReplicatedService{Replicas: &n}

    start := time.Now()
    c := newClient()
    res, err := c.ServiceCreate(context.Background(), service, options)
    if err != nil {
        panic(err)
    }

    var i uint64
    for i = 0; i < 20*n; i++ {
        var ops types.TaskListOptions
        c = newClient()
        tasks, err := c.TaskList(context.Background(), ops)
        if err != nil {
            panic(err)
        }

        var running uint64 = 0
        for _, task := range tasks {
            if task.ServiceID == res.ID && task.Status.State == "running" {
                running += 1
            }
        }

        if running == n {
            break
        }

        time.Sleep(500 * time.Millisecond)
    }

    if i == 20*n {
        panic(fmt.Errorf("%d instances can not be scaled in %ds", n, 10*n))
    }

    total := time.Since(start)
    return total.Seconds()
}

func bench(requests int, n uint64, image string, args []string) {
    start := time.Now()
    timings := make([]float64, requests)

    for i := 0; i < requests; i++ {
        timings[i] = scale(n, image, args)
        fmt.Printf("[%3.f%%] %d/%d request done\n", float64(i+1)/float64(requests)*100, i+1, requests)
    }

    total := time.Since(start)
    mean, _ := stats.Mean(timings)
    p90th, _ := stats.Percentile(timings, 90)
    p99th, _ := stats.Percentile(timings, 99)

    fmt.Printf("\n")
    fmt.Printf("Time taken for tests: %.3fs\n", total.Seconds())
    fmt.Printf("Time per request: %.3fs [mean] | %.3fs [90th] | %.3fs [99th]\n", mean, p90th, p99th)
}

func main() {
    app := cli.NewApp()
    app.Name = "instance-scale-bench"
    app.Usage = "DaoCloud swarm-kit benchmarking tool"
    app.Version = "0.1"
    app.Author = "haipeng"
    app.Email = "haipeng.wu@daocloud.io"
    app.Flags = []cli.Flag{
        cli.IntFlag{
            Name:  "requests, r",
            Value: 1,
            Usage: "Number of requests to start for the benchmarking session.",
        },
        cli.IntFlag{
            Name:  "instances, n",
            Value: 1,
            Usage: "Number of service instances to start in each request.",
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
        bench(c.Int("requests"), uint64(c.Int("instances")), c.StringSlice("image")[0], c.Args())
    }

    app.Run(os.Args)
}
