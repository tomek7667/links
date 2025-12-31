package http

func cloneHistory(src []HistoryPoint) []HistoryPoint {
	if len(src) == 0 {
		return nil
	}
	out := make([]HistoryPoint, len(src))
	for i, h := range src {
		out[i] = h
		if h.Disks != nil {
			dm := make(map[string]float64, len(h.Disks))
			for k, v := range h.Disks {
				dm[k] = v
			}
			out[i].Disks = dm
		}
	}
	return out
}
