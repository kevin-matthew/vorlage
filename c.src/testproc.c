#include <stdlib.h>
#include <string.h>
#include <sys/types.h>
#include "processor-interface.h"

int test(const char *d) {
	perror(d);
	return 9;
}

const vorlage_proc_info vorlage_proc_startup() {

	vorlage_proc_info v = {0};
	v.description="this is a test. don't use it";
	v.inputprotoc=0;
	v.inputprotov = (const vorlage_proc_inputproto []){
			{
				.name="logme",
				.description="logs it",
			},
	};
	v.streaminputprotoc = 0;
	v.streaminputprotov = (const vorlage_proc_inputproto []){
		{
			.name="logmestream",
			.description="outputs the stream in log format",
		}
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

typedef struct {
	char sizebuffer[30];
	vorlage_proc_action actionv[1];
} request_context;

const vorlage_proc_actions  vorlage_proc_onrequest(const vorlage_proc_requestinfo rinfo, void **context)
{
    /*const char *logme=rinfo.inputv[0];
	//fprintf(stderr, "hi I'm being logged from file request %s: %s\n", rinfo.filepath, logme);

	void *stream = rinfo.streaminputv[0];
	int n;
	int bufsize = 2;
	size_t totalsize = 0;
	char buf[bufsize];
	do {
		n = vorlage_stream_read(stream, buf, bufsize);
		for(int j = 0; j < n; j++) {
			//fputc(buf[j], stderr);
			totalsize ++;
		}
	}while(n > 0);
	*/
	request_context *reqcontx = malloc(sizeof(request_context));
	memset(reqcontx, 0, sizeof(request_context));
	//int datac = sprintf(reqcontx->sizebuffer, "X-Stream-Input-Was-Size: %ld", totalsize);
	int datac = sprintf(reqcontx->sizebuffer, "X-Stream-Input-Was-Size: %d", 69);
	reqcontx->actionv[0] = (vorlage_proc_action){
				.action = VORLAGE_PROC_ACTION_HTTPHEADER,
				.data   = (void *)(reqcontx->sizebuffer),
				.datac  = datac,
			};

	*context = reqcontx;
	//fprintf(stderr, "%s [%d]\n", reqcontx->sizebuffer, datac);
	
	vorlage_proc_actions v = {
		.actionc = 1,
		.actionv = reqcontx->actionv,
	};
	return v;
};

typedef struct {
	char *buffer;
	size_t pos;
} customstream;
int vorlage_proc_definer_read(void *definer, char *buf, size_t size) {
	customstream *c = (customstream *)definer;
	if(c->buffer[c->pos] == '\0') {
		return -2;
	}
	int i;
	for(i=0; i < size && c->buffer[i+c->pos] != '\0'; i++) {
		buf[i] = c->buffer[i+c->pos];
	}
	c->pos += i;
	return i;
}

int vorlage_proc_definer_reset(void *cookie) {
	customstream *c = (customstream *)cookie;
	c->pos = 0;
	return 0;
}
int vorlage_proc_definer_close(void *cookie) {
	customstream *c = (customstream *)cookie;
	free(c->buffer);
	free(c);
	return 0;
}


void  *vorlage_proc_define(const vorlage_proc_defineinfo  dinfo, void *context){
	const char *whattoecho = dinfo.inputv[0];
	const char *prefix     = "this is what you're echoing: ";

	// dealloc'd in cust_close (called in voralge)
	char *newstr = malloc(strlen(whattoecho) + strlen(prefix) + 1);
	strcat(newstr, prefix);
	strcat(&(newstr[strlen(whattoecho)]), whattoecho);
	customstream *c = malloc(sizeof(customstream));
	c->buffer = newstr;
	c->pos    = 0;
	return c;
};


void vorlage_proc_onfinish (const vorlage_proc_requestinfo rinfo, void  *context) {
	request_context *ctx = (request_context *)(context);
	free(ctx);
	//fprintf(stderr, "vorlage_proc_onfinish called\n");
};


int vorlage_proc_shutdown()
{
	fprintf(stderr, "vorlage_proc_shutdown called\n");
	return 0;
};

