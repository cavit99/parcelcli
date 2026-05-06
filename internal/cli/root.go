package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cavit99/parcelcli/internal/carriers/evri"
	"github.com/cavit99/parcelcli/internal/carriers/royalmail"
	"github.com/cavit99/parcelcli/internal/model"
	"github.com/cavit99/parcelcli/internal/watch"
	"github.com/spf13/cobra"
)

var jsonOut bool
var postcode, carrier, chromePath string
var timeout time.Duration
var debug bool

func NewRoot() *cobra.Command {
	root := &cobra.Command{Use: "parcelcli", Short: "Track parcels from the terminal", SilenceUsage: true}
	root.PersistentFlags().BoolVar(&jsonOut, "json", false, "emit stable JSON")
	root.PersistentFlags().StringVar(&chromePath, "chrome", "", "Chrome executable path for browser-backed carriers")
	root.PersistentFlags().DurationVar(&timeout, "timeout", 35*time.Second, "carrier request timeout")
	root.PersistentFlags().BoolVar(&debug, "debug", false, "keep/show extra debugging where supported")
	root.AddCommand(trackCmd(), detectCmd(), doctorCmd(), watchCmd())
	return root
}

func trackCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "track TRACKING_NUMBER", Short: "Track one parcel", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if carrier == "" {
			carrier = "evri"
		}
		res, err := runTrack(cmd.Context(), carrier, args[0], postcode)
		if err != nil {
			return err
		}
		return printResult(res)
	}}
	cmd.Flags().StringVar(&carrier, "carrier", "evri", "carrier slug (currently: evri, royalmail)")
	cmd.Flags().StringVar(&postcode, "postcode", "", "destination postcode when required")
	return cmd
}

func detectCmd() *cobra.Command {
	return &cobra.Command{Use: "detect TRACKING_NUMBER", Short: "Suggest possible carriers", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		number := strings.ToUpper(strings.ReplaceAll(args[0], " ", ""))
		candidates := []map[string]any{{"carrier": "evri", "confidence": "possible", "requires": []string{"postcode"}}}
		if looksRoyalMail(number) {
			candidates = append([]map[string]any{{"carrier": "royalmail", "confidence": "likely", "requires": []string{}}}, candidates...)
		}
		out := map[string]any{"tracking_number": args[0], "candidates": candidates}
		return printJSONOrText(out, fmt.Sprintf("Possible carriers: %s", carrierNames(candidates)))
	}}
}
func doctorCmd() *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Check local readiness", RunE: func(cmd *cobra.Command, args []string) error {
		chrome := chromePath
		if chrome == "" {
			chrome = "auto"
		}
		out := map[string]any{"ready": true, "carriers": map[string]any{"evri": map[string]any{"method": "browser", "chrome": chrome, "requires": []string{"postcode"}}, "royalmail": map[string]any{"method": "browser", "chrome": chrome, "requires": []string{}}}, "watch_state": watch.Path()}
		return printJSONOrText(out, "parcelcli is ready. Evri and Royal Mail use headless Chrome; Evri requires --postcode.")
	}}
}

func watchCmd() *cobra.Command {
	wc := &cobra.Command{Use: "watch", Short: "Manage local parcel watches"}
	wc.AddCommand(watchAddCmd(), watchListCmd(), watchRunCmd(), watchRemoveCmd())
	return wc
}
func watchAddCmd() *cobra.Command {
	var label string
	cmd := &cobra.Command{Use: "add TRACKING_NUMBER", Short: "Add a parcel watch", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if carrier == "" {
			carrier = "evri"
		}
		if postcode == "" && carrier == "evri" {
			return fmt.Errorf("evri watches require --postcode")
		}
		st, err := watch.Load()
		if err != nil {
			return err
		}
		item := watch.Item{ID: watch.NewID(carrier, args[0]), Carrier: carrier, TrackingNumber: args[0], Postcode: strings.ToUpper(strings.ReplaceAll(postcode, " ", "")), Label: label, AddedAt: time.Now().UTC().Format(time.RFC3339)}
		st.Items = append(st.Items, item)
		if err := watch.Save(st); err != nil {
			return err
		}
		return printJSONOrText(item, fmt.Sprintf("Added watch %s for %s", item.ID, item.TrackingNumber))
	}}
	cmd.Flags().StringVar(&carrier, "carrier", "evri", "carrier slug")
	cmd.Flags().StringVar(&postcode, "postcode", "", "destination postcode")
	cmd.Flags().StringVar(&label, "label", "", "human label")
	return cmd
}
func watchListCmd() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List parcel watches", RunE: func(cmd *cobra.Command, args []string) error {
		st, err := watch.Load()
		if err != nil {
			return err
		}
		return printJSONOrText(st, fmt.Sprintf("%d watch(es)", len(st.Items)))
	}}
}
func watchRemoveCmd() *cobra.Command {
	return &cobra.Command{Use: "remove ID", Short: "Remove a parcel watch", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		st, err := watch.Load()
		if err != nil {
			return err
		}
		out := st.Items[:0]
		removed := false
		for _, it := range st.Items {
			if it.ID == args[0] {
				removed = true
				continue
			}
			out = append(out, it)
		}
		st.Items = out
		if err := watch.Save(st); err != nil {
			return err
		}
		return printJSONOrText(map[string]any{"removed": removed, "id": args[0]}, fmt.Sprintf("removed=%v", removed))
	}}
}
func watchRunCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{Use: "run", Short: "Poll watches and emit changes", RunE: func(cmd *cobra.Command, args []string) error {
		st, err := watch.Load()
		if err != nil {
			return err
		}
		var changes []any
		for i, it := range st.Items {
			res, err := runTrack(cmd.Context(), it.Carrier, it.TrackingNumber, it.Postcode)
			if err != nil {
				changes = append(changes, map[string]any{"id": it.ID, "error": err.Error()})
				continue
			}
			h := watch.Hash(map[string]any{"status": res.Status, "last_event": res.LastEvent, "eta": res.EstimatedDelivery})
			if all || h != it.LastHash {
				changes = append(changes, map[string]any{"id": it.ID, "label": it.Label, "result": res, "changed": h != it.LastHash})
				st.Items[i].LastHash = h
			}
		}
		_ = watch.Save(st)
		return printJSONOrText(map[string]any{"changes": changes}, fmt.Sprintf("%d change(s)", len(changes)))
	}}
	cmd.Flags().BoolVar(&all, "all", false, "emit unchanged watches too")
	return cmd
}

func runTrack(ctx context.Context, c, number, pc string) (*model.Result, error) {
	switch c {
	case "evri":
		return evri.Tracker{}.Track(ctx, model.TrackRequest{TrackingNumber: number, Postcode: pc, ChromePath: chromePath, Timeout: timeout, Debug: debug})
	case "royalmail", "royal-mail", "rm":
		return royalmail.Tracker{}.Track(ctx, model.TrackRequest{TrackingNumber: number, Postcode: pc, ChromePath: chromePath, Timeout: timeout, Debug: debug})
	default:
		return nil, fmt.Errorf("unsupported carrier %q (currently implemented: evri, royalmail)", c)
	}
}
func printResult(r *model.Result) error {
	if jsonOut {
		return printJSON(r)
	}
	if r.LastEvent != nil {
		fmt.Printf("%s · %s · %s\n%s\n", strings.ToUpper(r.Carrier), r.Status, r.LastEvent.Time, r.LastEvent.Text)
	} else {
		fmt.Printf("%s · %s\n%s\n", strings.ToUpper(r.Carrier), r.Status, r.StatusText)
	}
	return nil
}
func printJSONOrText(v any, text string) error {
	if jsonOut {
		return printJSON(v)
	}
	fmt.Fprintln(os.Stdout, text)
	return nil
}
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func looksRoyalMail(number string) bool {
	if len(number) < 9 || len(number) > 27 {
		return false
	}
	if len(number) == 13 && strings.HasSuffix(number, "GB") {
		return true
	}
	digits := 0
	for _, r := range number {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	return digits >= 12 && digits == len(number)
}

func carrierNames(candidates []map[string]any) string {
	var names []string
	for _, c := range candidates {
		if name, ok := c["carrier"].(string); ok {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}
