#ifndef VORLAGE_PROCESSORS_H_
#define VORLAGE_PROCESSORS_H_ 1
#include <stdint.h>
#include <stdio.h>
#include "vorlage.h"

const int vorlage_proc_interfaceversion = 0x0;

/*
 * All structs that handle the input request scheme (proto) and handle
 * the actual input.
 *
 * Input (vorlage_proc_input, vorlage_proc_streaminput) are structs that hold
 * what the user had inputed. Input is simply an array of input that
 * is associtative to the proto type that was supplied.
 * 
 * Input prototypes (vorlage_proc_inputproto) are simply an array of input
 * name, and input description.
 *
 */
typedef struct {
	// an array of structs with each item holding the input's name
	// (aka argument name) and a description of what the input is.
	int                                                inputc;
	struct{const char *name; const char *description;} *inputv;
} vorlage_proc_inputproto;
typedef struct {
	// back-reference pointer
	vorlage_proc_inputproto *proto;

	
} vorlage_proc_input;
typedef struct {
	// back-reference pointer
	vorlage_proc_inputproto *proto;

	// array of file stream pointers. note that the processor must
	// close the stream, vorlage will not close the stream. All stream
	// will be read-only.
	//
	// note: streams are not guaranteed to be seekable.
	int    streamc;
	FILE **streamv;
} vorlage_proc_streaminput;

/*
 * The vorlage_proc_variable struct is used to by the processor to
 * tell vorlage what variable it has availabe. It's also used in 
 * vorlage_proc_defineinfo as a pointer so that the processor can see
 * the exact reference of what variable needs to be defined.
 */
typedef struct {
	// the processor-variable name of the variable
	const char *name;
	
	// Describe what this processor does and why someone would need
	// it.
	const char *description;

	// Specify what input field names this variable needs during the
	// output phase (can be nil)
	const vorlage_proc_inputproto       *inputproto;
	const vorlage_proc_inputproto *streaminputproto;	
} vorlage_proc_variable;
	

/*
 * rid is a globally unique request id that is generated by vorlage.
 */
typedef uint64_t rid;

/*
 * vorlage_proc_info is given to vorlage the instant a processor is loaded.
 */
typedef struct {
	// Describe what this processor does and why someone would need
	// it.
	const char *description;

	// Specify what input field names this processor needs during the
	// request phase (can be nil)
	vorlage_proc_inputproto       *inputproto;
	vorlage_proc_inputproto *streaminputproto;


	// an array of variables that this processor provides to
	// documents.
	vorlage_proc_variable *variablesv;
	int                    variablesc;
} vorlage_proc_info;

/*
 * vorlage_proc_requestinfo is provided to processors
 */
typedef struct {
	// procinfo is a pointer to the procinfo that was returned by the
	// processor's vorlage_startup function.
	const vorlage_proc_info *procinfo;

	// nullterm string of the filepath that's being requested.
	const char *filepath;

	// the input that reflects the scheme provided by inputproto.
	const vorlage_proc_input *input;
	const vorlage_proc_streaminput *streaminput;

	// request id
	rid rid;
	
} vorlage_proc_requestinfo;

/*
 * vorlage_proc_action (and its array wrapper vorlage_proc_actions)
 * is a struct to which can specify actions to perform before the
 * request is even executed.
 * The list of actions can be found in vorlage_proc_actionenum along
 * with the each action's documentation.
 */
enum vorlage_proc_actionenum {

	// The processor has hit a critical error that is it's own fault.
	// This action will stop the request. vorlage_proc_action.data can
	// be set to a null-terminated string that will be shown to the
	// user.
	VORLAGE_PROC_ACTION_CRITICAL,
	
	// The processor recongizes that the request is a violation of the
	// access granted to the user. vorlage_proc_action.data can be
	// set to a null-term string that will be shown to the user.
	// tip: use this in conjunction with VORLAGE_PROC_ACTION_SEE to
	//      invoke a redirect to a longin page.
	VORLAGE_PROC_ACTION_ACCESSFAIL,

	// The processor request that the user see another
	// file. vorlage_proc_action.data must be set to a file path to
	// which the user will be directed to.
	VORLAGE_PROC_ACTION_SEE,

	
	/**** HTTP only ****/
	
	// The processor will set a cookie to the user's
	// browser. vorlage_proc_action.data must be a null-term string
	// that is a valid cookie syntax defined in rfc6265 section 4.1.1.
	// vorlage_proc_action.data must NOT include header name. (don't
	// dictate the "Set-Cookie:" part but dictate everything after
	// that)
	VORLAGE_PROC_ACTION_HTTPCOOKIE,
};
// action struct (see above enum)
typedef struct {
	// action is an int/enum that is equal to any item found in the
	// aciton list (see enum vorlage_proc_actionenum)
	enum vorlage_proc_actionenum action;

	// data is arbitrary data that is context-specific to whatever
	// action was set. So see the aformentioned action list.
	void *data;
} vorlage_proc_action;
// action array struct (see above struct)
typedef struct {
	vorlage_proc_action *actionv;
	int                  actionc;
} vorlage_proc_actions;


/*
 * vorlage_proc_defineinfo is a struct that holds all the information
 * needed to provide a processor sufficent data to define a variable.
 */
typedef struct {
	// the request info
	const vorlage_proc_requestinfo *requestinfo;

	// the variable which needs to be defined
	const vorlage_proc_variable *procvar;

	// the input that reflects the scheme provided by inputproto.
	const vorlage_proc_input       *input;
	const vorlage_proc_streaminput *streaminput;
} vorlage_proc_defineinfo;


/*
 * Processors use vorlage_proc_definer structs to provide their
 * definitions to vorlage during the outputting phase.
 */
typedef struct {
	
	// all definers are treated as streams. Make your stream by using
	// either fopen(3) or fmemopen(3). vorlage only needs read access.
	// vorlage will close filedes when it's done with it.
	FILE *filedes;
	
} vorlage_proc_definer;


/*
 * When vorlage shuts down, it collects vorlage_proc_exitinfo from
 * all the processors for logging reasons. As of now, there's no
 * actionable items that processors can invoke during shutdown.
 */
typedef struct {
	// anything not 0 will be seen as an error.
	int exitstatus;

	// if existstatus != 0, error will be used to elaborate what
	// had happened.
	const char *error;
} vorlage_proc_exitinfo;

/*
 * if you're making a processor, you must define these following 
 * functions which deal with the overall life cycle of a request.
 *
 * Vorlage will call these functions when this processor is loaded.
 */
const vorlage_proc_info     vorlage_proc_startup  ();
const vorlage_proc_actions  vorlage_proc_onrequest(const vorlage_proc_requestinfo *rinfo);
const vorlage_proc_definer  vorlage_proc_define   (const vorlage_proc_defineinfo  *dinfo);
const vorlage_proc_exitinfo vorlage_proc_shutdown ();


#endif /* VORLAGE_PROCESSORS_H_ */
