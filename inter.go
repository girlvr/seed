package seed

import (
	"context"

	"github.com/glvd/seed/model"
	"github.com/go-xorm/xorm"
	httpapi "github.com/ipfs/go-ipfs-http-client"
)

// State ...
type State int

// State ...
const (
	StateWaiting State = iota
	StateRunning
	StateStop
)

// Stepper ...
type Stepper int

// StepperNone ...
const (
	// StepperNone ...
	StepperNone Stepper = iota

	//StepperRDatabase ...
	//StepperRDatabase
	//StepperDatabase ...
	StepperDatabase

	//StepperAPI ...
	StepperAPI
	//StepperSlice ...
	StepperSlice
	// StepperProcess ...
	StepperProcess
	// StepperInformation ...
	StepperInformation
	// StepperMoveInfo ...
	StepperMoveInfo

	// StepperMoveproc ...
	StepperMoveproc
	// StepperTransfer ...
	StepperTransfer
	// StepperPin ...
	StepperPin
	// StepperCheck ...
	StepperCheck
	// StepperUpdate ...
	StepperUpdate
	// StepperMax ...
	StepperMax
)

// Seeder ...
type Seeder interface {
	Start()
	Wait()
	Stop()
	PushTo(stepper Stepper, v interface{}) error
	GetThread(stepper Stepper) Threader
	SetThread(stepper Stepper, threader Threader)
	HasThread(stepper Stepper) bool
	//SetBaseThread(stepper Stepper, threader BaseThreader)
	//IsBase(stepper Stepper) bool
	//SetNormalThread(stepper Stepper, threader Threader)
	//IsNormal(stepper Stepper) bool
	Register(ops ...Optioner)
}

// Initer ...
type Initer interface {
	Init()
}

//Optioner set option
type Optioner interface {
	Option(Seeder)
}

// DatabaseCallbackFunc ...
type DatabaseCallbackFunc func(database *Database, eng *xorm.Engine, v interface{}) (e error)

// DatabaseCaller ...
type DatabaseCaller interface {
	Call(database *Database, eng *xorm.Engine) (e error)
}

// APICallbackFunc ...
type APICallbackFunc func(api *API, api2 *httpapi.HttpApi, v interface{}) (e error)

// APICaller ...
type APICaller interface {
	Call(*API, *httpapi.HttpApi) error
}

// ProcessCallbackFunc ...
type ProcessCallbackFunc func(*Process, string) error

// ProcessCaller ...
type ProcessCaller interface {
	Call(process *Process) error
}

// InformationCallbackFunc ...
type InformationCallbackFunc func(information *Information, v *model.Video)

// Threader ...
type Threader interface {
	Runnable
	Pusher
	BeforeRun(seed Seeder)
	AfterRun(seed Seeder)
}

// Base ...
type Base interface {
	State() State
	Done() <-chan bool
}

// BaseThreader ...
type BaseThreader interface {
	Threader
	Base
}

// Async ...
type Async interface {
	NeedWait() bool
	IsRunning() bool
	Rerun()
	Stop()
}

// Runnable ...
type Runnable interface {
	Run(context.Context)
}

// Pusher ...
type Pusher interface {
	Push(interface{}) error
}
