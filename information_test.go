package seed

import "testing"

// TestInformation ...
func TestInformation(t *testing.T) {
	seed := NewSeed(Information("D:\\videoall\\video.json", InfoFlagBSON), DatabaseOption("sqlite3", "test2.db"), Update(UpdateStatusAdd))
	seed.Workspace = "D:\\videoall"
	seed.AfterInit(SyncDatabase(), ShowSQLOption(), ShowExecTimeOption())
	seed.Start()

	seed.Wait()
}