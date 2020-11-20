#ifndef VORLAGE_PROCESSORS_INTERFACE_H_
#define VORLAGE_PROCESSORS_INTERFACE_H_ 1
#include "processors.h"

/*
 * if you're making a processor, you must define these following 
 * functions which deal with the overall life cycle of a request.
 *
 * Vorlage will call these functions when this processor is loaded.
 *
 * note: these functions are marked inline for the purpose of forcing you to
 *       define them, as the compiler will fail if inline funcs are
 *       left undefined.
 */
inline vorlage_proc_info     vorlage_proc_startup  ();
inline vorlage_proc_actions  vorlage_proc_onrequest(const vorlage_proc_requestinfo rinfo);
inline int                   vorlage_proc_define   (const vorlage_proc_defineinfo  dinfo);
inline vorlage_proc_exitinfo vorlage_proc_shutdown ();


#endif /* VORLAGE_PROCESSORS_INTERFACE_H_ */
