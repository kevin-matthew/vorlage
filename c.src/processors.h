#ifndef VORLAGE_PROCESSORS_H_
#define VORLAGE_PROCESSORS_H_ 1
#include "vorlage.h"

const int vorlage_proc_interfaceversion = 0x0

/*
 * vorlage_proc_info is given to vorlage the instant a processor is loaded.
 */
typedef struct {
	// Describe what this processor does and why someone would need
	// it.
	const char *description;

	// Specify what input field names this processor needs during the
	// request phase (can be nil)
	vorlage_inputproto       *inputproto;
	vorlage_streaminputproto *streaminputproto;


	// an array of variables that this processor provides to
	// documents.
	vorlage_variable *variablesv
	int               variablesc
} vorlage_proc_info;

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
	// aciton list (see VORLAGE_PROC_
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





typedef struct {

} vorlage_proc_definer;
typedef struct {

} vorlage_proc_exitinfo;

// if you're making a processor, you must define these:
const vorlage_proc_info     vorlage_startup  ();
const vorlage_proc_actions  vorlage_onrequest(const vorlage_requestinfo *rinfo);
const vorlage_proc_definer  vorlage_define   (const vorlage_defineinfo  *dinfo);
const vorlage_proc_exitinfo vorlage_shutdown ();




#endif /* VORLAGE_PROCESSORS_H_ */
