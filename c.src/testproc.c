#include <stdio.h>
#include "processor-interface.h"

int test(const char *d) {
	perror(d);
	return 9;
}

const vorlage_proc_info vorlage_proc_startup() {

	vorlage_proc_info v = {0};
	v.description="this is a test. don't use it";
	v.inputprotoc=1;
	v.inputprotov = (const vorlage_proc_inputproto []){
			{
				.name="logme",
				.description="logs it",
			},
	};
	v.variablesc = 1;
	v.variablesv = (const vorlage_proc_variable []){
			{
				.name="echo",
				.description="echos the message",
				.inputprotoc = 1,
				.inputprotov = (const vorlage_proc_inputproto []){
					{
						.name="echotext",
						.description="the text to which to echo",
					}},
			},
	};
	return v;
}

const vorlage_proc_actions  vorlage_proc_onrequest(const vorlage_proc_requestinfo rinfo)
{
    const char *logme=rinfo.inputv[0];
	fprintf(stderr, "hi I'm being logged from file request %s: %s\n", rinfo.filepath, logme);
	vorlage_proc_actions v = {
		.actionc = 1,
		.actionv = (const vorlage_proc_action [])
		{
			{
				.action = VORLAGE_PROC_ACTION_HTTPCOOKIE,
				.data   = (void *)("lol"),
				.datac  = 3,
			},
		},
	};
	return v;
};


const vorlage_proc_definer  vorlage_proc_define(const vorlage_proc_defineinfo  dinfo){
vorlage_proc_definer v = {0};
return v;
};


const vorlage_proc_exitinfo vorlage_proc_shutdown()
{
vorlage_proc_exitinfo v ={0};
return v;
};

