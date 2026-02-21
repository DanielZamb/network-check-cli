package compare

import (
	"encoding/json"
	"fmt"
	"netcheck/internal/model"
	"os"
	"sort"
	"text/tabwriter"
)

type Item struct {
	ID             string       `json:"id"`
	BeforeStatus   model.Status `json:"before_status"`
	AfterStatus    model.Status `json:"after_status"`
	BeforeDuration int64        `json:"before_duration_ms"`
	AfterDuration  int64        `json:"after_duration_ms"`
}

type Diff struct {
	BeforeScore int    `json:"before_score"`
	AfterScore  int    `json:"after_score"`
	Items       []Item `json:"items"`
}

func Load(path string) (model.Report, error) {
	var r model.Report
	b, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(b, &r)
	return r, err
}

func Build(before, after model.Report) Diff {
	bm := map[string]model.CheckResult{}
	for _, c := range before.Checks {
		bm[c.ID] = c
	}
	items := make([]Item, 0, len(after.Checks))
	for _, c := range after.Checks {
		b := bm[c.ID]
		items = append(items, Item{ID: c.ID, BeforeStatus: b.Status, AfterStatus: c.Status, BeforeDuration: b.DurationMS, AfterDuration: c.DurationMS})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return Diff{BeforeScore: before.Score, AfterScore: after.Score, Items: items}
}

func WriteTable(path string, d Diff) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	tw := tabwriter.NewWriter(f, 0, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "ID\tBEFORE\tAFTER\tBEFORE_MS\tAFTER_MS\n")
	for _, it := range d.Items {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\n", it.ID, it.BeforeStatus, it.AfterStatus, it.BeforeDuration, it.AfterDuration)
	}
	fmt.Fprintf(tw, "\nScore\t%d\t%d\t\t\n", d.BeforeScore, d.AfterScore)
	return tw.Flush()
}
