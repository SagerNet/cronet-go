package cronet

type Result int

const (
	ResultSuccess                                           Result = 0
	ResultIllegalArgument                                   Result = -100
	ResultIllegalArgumentStoragePathMustExist               Result = -101
	ResultIllegalArgumentInvalidPin                         Result = -102
	ResultIllegalArgumentInvalidHostname                    Result = -103
	ResultIllegalArgumentInvalidHttpMethod                  Result = -104
	ResultIllegalArgumentInvalidHttpHeader                  Result = -105
	ResultIllegalState                                      Result = -200
	ResultIllegalStateStoragePathInUse                      Result = -201
	ResultIllegalStateCannotShutdownEngineFromNetworkThread Result = -202
	ResultIllegalStateEngineAlreadyStarted                  Result = -203
	ResultIllegalStateRequestAlreadyStarted                 Result = -204
	ResultIllegalStateRequestNotInitialized                 Result = -205
	ResultIllegalStateRequestAlreadyInitialized             Result = -206
	ResultIllegalStateRequestNotStarted                     Result = -207
	ResultIllegalStateUnexpectedRedirect                    Result = -208
	ResultIllegalStateUnexpectedRead                        Result = -209
	ResultIllegalStateReadFailed                            Result = -210
	ResultNullPointer                                       Result = -300
	ResultNullPointerHostname                               Result = -301
	ResultNullPointerSha256Pins                             Result = -302
	ResultNullPointerExpirationDate                         Result = -303
	ResultNullPointerEngine                                 Result = -304
	ResultNullPointerUrl                                    Result = -305
	ResultNullPointerCallback                               Result = -306
	ResultNullPointerExecutor                               Result = -307
	ResultNullPointerMethod                                 Result = -308
	ResultNullPointerHeaderName                             Result = -309
	ResultNullPointerHeaderValue                            Result = -310
	ResultNullPointerParams                                 Result = -311
	ResultNullPointerRequestFinishedInfoListenerExecutor    Result = -312
)
