package seed_test

import (
	"testing"

	"github.com/glvd/seed"
	_ "github.com/mattn/go-sqlite3"
)

// TestGetFiles ...
func TestGetFiles(t *testing.T) {

}

// TestProcess ...
func TestProcess(t *testing.T) {
	seeder := seed.NewSeed()
	proc := seed.NewProcess()
	seeder.Register(proc)

	seeder.Start()

	seeder.Wait()

}

// TestName ...
func TestName(t *testing.T) {
	//t.Log(onlyNo("file-09-B.name"))
	//t.Log(onlyNo("file-09B.name"))
	//t.Log(onlyNo("file-001R"))
	//t.Log(onlyNo(".file"))
	//t.Log(onlyNo("."))
	//t.Log(onlyNo(""))
	//t.Log(NumberIndex("file-09"))
	//t.Log(NumberIndex("file-09-C"))
	//t.Log(NumberIndex("file-09-B"))
	//t.Log(NumberIndex("file-09-A"))
}
