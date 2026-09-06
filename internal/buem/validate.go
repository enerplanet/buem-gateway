package buem

// ValidateSingle reports whether a single-building request is complete
// enough for RunSingle to attempt, without ever calling BuEM. It runs the
// exact same pre-flight as the run path — TaskFromBuilding, which checks
// envelope, weather, geometry and start_date and builds the BuEM feature but
// never sends it — so /api/v1/buem/validate and /api/v1/buem/building can
// never disagree on whether a request is well-formed. A nil return means
// every check buem-gateway performs passed; BuEM may still reject the
// request at run time.
func ValidateSingle(in BuildingInput, startDate, endDate string, resolution int) error {
	_, err := TaskFromBuilding(in, startDate, endDate, resolution, "")
	return err
}
