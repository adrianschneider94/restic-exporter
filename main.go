package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/cli"
)

type ResticCollector struct {
	config Config
}

func NewResticCollector(configPath string) *ResticCollector {
	p := new(ResticCollector)
	p.Load(configPath)
	err := p.Validate()
	if err != nil {
		panic(err)
	}
	return p
}

func (cc *ResticCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(cc, ch)
}

func (cc *ResticCollector) Collect(ch chan<- prometheus.Metric) {
	group := sync.WaitGroup{}

	for _, targetConfig := range cc.config.Targets {
		group.Add(1)
		go CollectTarget(targetConfig, cc.config.Global, ch, &group)
	}
	group.Wait()
}

func main() {
	app := cli.NewApp()
	app.Name = "restic_exporter"
	app.Usage = "Export restic metrics for prometheus"
	app.Version = "0.0.1-alpha.1"
	app.EnableBashCompletion = true
	app.Author = "Adrian Schneider"
	app.Copyright = "MIT"
	app.Email = "post@adrian-schneider.de"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:     "config, c",
			FilePath: "config.yaml",
			Usage:    "specifies the `path` of the config",
		},
		cli.IntFlag{
			Name:  "port, p",
			Value: 9635,
			Usage: "specifies the `port` the exporter should bind to",
		},
		cli.StringFlag{
			Name:  "host",
			Usage: "sets the host address",
			Value: "0.0.0.0",
		},
	}

	app.Action = func(c *cli.Context) error {
		registry := prometheus.NewPedanticRegistry()

		registry.MustRegister(
			NewResticCollector(c.String("config")),
		)

		http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
		log.Fatal(http.ListenAndServe(c.String("host")+":"+c.String("port"), nil))
		return nil
	}

	app.Commands = []cli.Command{
		{
			Name:  "validate",
			Usage: "Validate a config file",
			Action: func(c *cli.Context) error {
				fmt.Println("Not implemented yet.")
				return nil
			},
			ArgsUsage: "[config.yaml]",
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}
