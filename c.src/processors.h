#ifndef VORLAGE_PROCESSORS_H_
#define VORLAGE_PROCESSORS_H_ 1
#include "vorlage.h"


// below are structs that must be returned by processors during the vorlage
// request cycle.
typedef struct {

} vorlage_proc_info;
typedef struct {

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
