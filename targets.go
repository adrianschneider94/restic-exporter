package main

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type RestoreSizeStats struct {
	TotalSize      int `json:"total_size"`
	TotalFileCount int `json:"total_file_count"`
}

var (
	integrityMetric = prometheus.NewDesc(
		"restic_repository_integrity",
		"The integrityMetric of the repository.",
		[]string{"repository", "alias"}, nil,
	)
	totalSnapshotsMetric = prometheus.NewDesc(
		"restic_repository_total_snapshots",
		"The number of snapshots in this repository",
		[]string{"repository", "alias"}, nil,
	)
	restoreSizeBytesMetric = prometheus.NewDesc(
		"restic_repository_restore_size",
		"The restore size in bytes.",
		[]string{"repository", "alias"}, nil,
	)
	restoreSizeFileCountMetric = prometheus.NewDesc(
		"restic_repository_restore_file_count",
		"The number of files.",
		[]string{"repository", "alias"}, nil,
	)
	filesByContentsBytesMetric = prometheus.NewDesc(
		"restic_repository_files_by_content_size",
		"The restore size in bytes.",
		[]string{"repository", "alias"}, nil,
	)
	filesByContentFileCountMetric = prometheus.NewDesc(
		"restic_repository_files_by_content_file_count",
		"The number of files.",
		[]string{"repository", "alias"}, nil,
	)
)

type Time time.Time

func (tt *Time) UnmarshalJSON(b []byte) error {
	var stringTime string
	err := json.Unmarshal(b, &stringTime)
	if err != nil {
		return err
	}

	jsonTime, err := time.Parse("2006-01-02T15:04:05.999999-07:00", stringTime)
	if err != nil {
		panic(err)
	}

	*tt = Time(jsonTime)
	return nil
}

func (tt Time) String() string {
	return time.Time(tt).String()
}

type Snapshot struct {
	Time     Time
	Tree     string
	Paths    []string
	Hostname string
	Username string
	Uid      int
	Gid      int
	Id       string
	Short_id string
	Tags     []string
}

func CollectTarget(config TargetConfig, globalConfig GlobalConfig, ch chan<- prometheus.Metric, group *sync.WaitGroup) {
	// Repository data
	// Check repository integrityMetric
	checkRepository := exec.Command("restic", "-r", config.Path, "check", "--no-lock")
	checkRepository.Env = append(os.Environ(), "RESTIC_PASSWORD="+config.Password)
	output, err := checkRepository.CombinedOutput()

	repositoryIsValid := 1.0
	if err != nil {
		repositoryIsValid = 0.0
		if strings.Contains(string(output), "repository is already locked") {
			repositoryIsValid = 0.5
		}
	}

	ch <- prometheus.MustNewConstMetric(
		integrityMetric,
		prometheus.GaugeValue,
		repositoryIsValid,
		config.Path,
		config.Alias,
	)

	// Restore size
	var totalSize int
	var totalFileCount int

	getRestoreSizeStats := exec.Command("restic", "-r", config.Path, "stats", "--mode", "restore-size", "--json", "--no-lock")
	getRestoreSizeStats.Env = append(os.Environ(), "RESTIC_PASSWORD="+config.Password)
	output, err = getRestoreSizeStats.CombinedOutput()

	if err != nil {
		if strings.Contains(string(output), "repository is already locked") {
			totalSize = -1
			totalFileCount = -1

		} else {
			panic(err)
		}
	} else {
		restoreSizeStats := new(RestoreSizeStats)
		err = json.Unmarshal(output, &restoreSizeStats)
		if err != nil {
			panic(err)
		}
		totalSize = restoreSizeStats.TotalSize
		totalFileCount = restoreSizeStats.TotalFileCount

		ch <- prometheus.MustNewConstMetric(
			restoreSizeBytesMetric,
			prometheus.GaugeValue,
			float64(totalSize),
			config.Path,
			config.Alias,
		)

		ch <- prometheus.MustNewConstMetric(
			restoreSizeFileCountMetric,
			prometheus.GaugeValue,
			float64(totalFileCount),
			config.Path,
			config.Alias,
		)
	}

	command := exec.Command("restic", "-r", config.Path, "stats", "--mode", "files-by-contents", "--json", "--no-lock")
	command.Env = append(os.Environ(), "RESTIC_PASSWORD="+config.Password)
	output, err = command.CombinedOutput()

	if err != nil {
		if strings.Contains(string(output), "repository is already locked") {
			totalSize = -1
			totalFileCount = -1

		} else {
			panic(err)
		}
	} else {
		restoreSizeStats := new(RestoreSizeStats)
		err = json.Unmarshal(output, &restoreSizeStats)
		if err != nil {
			panic(err)
		}
		totalSize = restoreSizeStats.TotalSize
		totalFileCount = restoreSizeStats.TotalFileCount

		ch <- prometheus.MustNewConstMetric(
			filesByContentsBytesMetric,
			prometheus.GaugeValue,
			float64(totalSize),
			config.Path,
			config.Alias,
		)

		ch <- prometheus.MustNewConstMetric(
			filesByContentFileCountMetric,
			prometheus.GaugeValue,
			float64(totalFileCount),
			config.Path,
			config.Alias,
		)
	}

	// Get groupBy
	var groupBy GroupBy
	if config.GroupBy.isSet {
		groupBy = config.GroupBy
	} else {
		groupBy = globalConfig.GroupBy
	}

	// Get list of snapshots
	getRestoreSizeStats = exec.Command("restic", "-r", config.Path, "snapshots", "--json", "--no-lock")
	getRestoreSizeStats.Env = append(os.Environ(), "RESTIC_PASSWORD="+config.Password)
	output, err = getRestoreSizeStats.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "repository is already locked") {

		} else {
			panic(err)
		}
	} else {
		var snapshots []Snapshot
		err = json.Unmarshal(output, &snapshots)
		if err != nil {
			fmt.Println(string(output))
			panic(err)
		}

		// Total number of snapshots
		ch <- prometheus.MustNewConstMetric(
			totalSnapshotsMetric,
			prometheus.GaugeValue,
			float64(len(snapshots)),
			config.Path,
			config.Alias,
		)

		// Group snapshots
		snapshotGroups, err := GroupSnapshots(snapshots, groupBy)
		if err != nil {
			panic(err)
		}

		for k, snapshots := range snapshotGroups {
			var snapshotGroupKey SnapshotGroupKey
			err := json.Unmarshal([]byte(string(k)), &snapshotGroupKey)
			if err != nil {
				panic(err)
			}
			CollectGroup(snapshotGroupKey, snapshots, groupBy, config, globalConfig, ch)
		}
	}

	group.Done()
}

func CollectGroup(groupKey SnapshotGroupKey, snapshots []Snapshot, groupBy GroupBy, config TargetConfig, globalConfig GlobalConfig, ch chan<- prometheus.Metric) {
	labels := make(map[string]string, 0)
	if groupBy.Host {
		labels["host"] = groupKey.Hostname
	}
	if groupBy.Tags {
		labels["tags"] = strings.Join(groupKey.Tags, ",")
	}
	if groupBy.Paths {
		labels["paths"] = strings.Join(groupKey.Paths, ",")
	}

	keys := make([]string, 0)
	values := make([]string, 0)
	for key, value := range labels {
		keys = append(keys, key)
		values = append(values, value)
	}

	labelValues := append([]string{config.Path, config.Alias}, values...)

	// Count repositories
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			"restic_group_total_snapshots",
			"",
			append([]string{"repository", "alias"}, keys...), nil,
		),
		prometheus.GaugeValue,
		float64(len(snapshots)),
		labelValues...
	)

	// Count hours with existing snapshots
	countIntervalsWithSnapshots(snapshots, time.Hour, "hours", keys, labelValues, ch)
	countIntervalsWithSnapshots(snapshots, 24*time.Hour, "days", keys, labelValues, ch)
	countIntervalsWithSnapshots(snapshots, 7*24*time.Hour, "weeks", keys, labelValues, ch)
	countIntervalsWithSnapshots(snapshots, 30*24*time.Hour, "months", keys, labelValues, ch)
	countIntervalsWithSnapshots(snapshots, 365*24*time.Hour, "years", keys, labelValues, ch)

}

func countIntervalsWithSnapshots(snapshots []Snapshot, duration time.Duration, durationLabel string, keys []string, labelValues []string, ch chan<- prometheus.Metric) {
	var currentTime = time.Now()
	var count int

	for {
		fmt.Println(count)
		end := currentTime.Add(time.Duration(-1 * count * int(duration)))
		start := end.Add(-1 * duration)
		foundOne := false
		for _, snapshot := range snapshots {
			snapshotTime := time.Time(snapshot.Time)
			fmt.Println("\n\n")
			fmt.Println("Snapshot", snapshotTime.Format(time.RFC822))
			fmt.Println("Start", start.Format(time.RFC822))
			fmt.Println("End", end.Format(time.RFC822))
			if start.Before(snapshotTime) && end.After(snapshotTime) {
				count++
				foundOne = true
				fmt.Println("Ja", count)
				break
			}
		}
		if !foundOne {
			break
		}
	}

	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(
			"restic_group_"+durationLabel+"_with_snapshots",
			"",
			append([]string{"repository", "alias"}, keys...), nil,
		),
		prometheus.GaugeValue,
		float64(count),
		labelValues...
	)
}
