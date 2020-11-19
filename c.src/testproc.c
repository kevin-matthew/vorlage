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

typedef struct {
	char *buffer;
	int pos;
} customstream;
static ssize_t cust_read(void *cookie, char *buf, size_t size) {
	// todo!!!! see man(3) fopencookie	
}
static int seek(void *cookie, off64_t *offset, int whence) {
	// todo!!!! see man(3) fopencookie
}
static int cust_close(void *cookie) {
	c = (customstream *)cookie;
	free(c->buffer);
	return 0;
}

const int  vorlage_proc_define(const vorlage_proc_defineinfo  dinfo){
	const char *whattoecho = dinfo.input[0];
	const char *prefix     = "this is what you're echoing:";

	// dealloc'd in cust_close (called in voralge)
	char *newstr = malloc(strlen(whattoecho) + strlen(prefix) + 1);
	strcat(newstr, prefix);
	strcat(&(newstr[strlen(whattoecho)]), prefix);
	customstream c = {
		.buffer = newstr,
		.pos = 0,
	};
	cookie_io_functions_t funcs = {
		.read = cust_read,
		.write = 0,
		.seek = cust_seek,
		.close = cust_close,
	};
	FILE *f = fopencookie(&c, 'r', funcs);
	return fileno(f);
};


const vorlage_proc_exitinfo vorlage_proc_shutdown()
{
vorlage_proc_exitinfo v ={0};
return v;
};

