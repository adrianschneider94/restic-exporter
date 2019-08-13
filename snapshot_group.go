package main

import (
	"encoding/json"
	"sort"
)

// SnapshotGroupKey is the structure for identifying groups in a grouped
// snapshot list. This is used by GroupSnapshots()
type SnapshotGroupKey struct {
	Hostname string   `json:"hostname"`
	Paths    []string `json:"paths"`
	Tags     []string `json:"tags"`
}

// GroupSnapshots takes a list of snapshots and a grouping criteria and creates
// a group list of snapshots.
func GroupSnapshots(snapshots []Snapshot, groupBy GroupBy) (map[string][]Snapshot, error) {
	// group by hostname and dirs
	snapshotGroups := make(map[string][]Snapshot)

	for _, sn := range snapshots {
		// Determining grouping-keys
		var tags []string
		var hostname string
		var paths []string

		if groupBy.Tags {
			tags = sn.Tags
			sort.StringSlice(tags).Sort()
		}
		if groupBy.Host {
			hostname = sn.Hostname
		}
		if groupBy.Paths {
			paths = sn.Paths
		}

		sort.StringSlice(sn.Paths).Sort()
		var k []byte
		var err error

		k, err = json.Marshal(SnapshotGroupKey{Tags: tags, Hostname: hostname, Paths: paths})

		if err != nil {
			return nil, err
		}
		snapshotGroups[string(k)] = append(snapshotGroups[string(k)], sn)
	}

	return snapshotGroups, nil
}
