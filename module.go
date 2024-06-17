package nerv

// Interface used in nerv engine to manage modules
// loaded in by the user
type Module interface {
	GetName() string
	RecvModulePane(pane *ModulePane)
	Start() error
	Shutdown()
}

// Interface for module to take action without
// access to engine object
type ModulePane struct {

	// Retrieve whatever meta-data the users stored
	// with the module
	GetModuleMeta func(moduleName string) interface{}

	// Subscribe a set of consumers to a topic. If register is TRUE, then the
	// engine will be momentarily locked to ensure that the consumer is registered
	// as subscription requires a registered consumer. It is safe to always pass TRUE
	// but it map cause performance overhead if its called a lot as such
	SubscribeTo func(topic string, consumers []Consumer, register bool) error

	// Permits module to submit raw data as an event onto
	// its associated topics as its own producer, where consumers
	// registered to that function will recieve it
	SubmitTo func(topic string, data interface{})

	// Place an event onto the bus from the module that may or may
	// not go to consumers of the module. This function is useful
	// for fowarding events through a module without obfuscating
	// the original event
	SubmitEvent EventRecvr
}
