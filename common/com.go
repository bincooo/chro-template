package common

var initFunctions = make([]func(), 0)

func InitCommon() {
	for _, apply := range initFunctions {
		apply()
	}
}

func AddInitialized(apply func()) {
	initFunctions = append(initFunctions, apply)
}
