package download

// Test seams for injecting active download tasks into the global status map.
// These mirror the ext.SetEntClientForTest pattern: production code stays
// unaware of tests, but endpoint tests in other packages (e.g. pkg/grpc
// GetStorageStats) can simulate in-progress (temp) downloads without starting a
// real download.

// StoreActiveProgressForTest injects a Progress into the global status map used
// by DownloadStatus().
func StoreActiveProgressForTest(p *Progress) {
	statusMap.Store(p.TaskID, p)
}

// ClearActiveProgressForTest empties the global status map, isolating tests
// that inject tasks via StoreActiveProgressForTest.
func ClearActiveProgressForTest() {
	statusMap.Range(func(k, _ any) bool {
		statusMap.Delete(k)
		return true
	})
}
