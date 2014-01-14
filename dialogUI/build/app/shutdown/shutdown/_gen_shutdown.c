
#include <JavaScriptCore/JSContextRef.h>
#include <JavaScriptCore/JSStringRef.h>
#include "jsextension.h"
#include <glib.h>
#include <glib-object.h>

extern void* get_global_webview();


extern void shutdown_quit(JSData*);
static JSValueRef __quit__ (JSContextRef noused_context,
                            JSObjectRef function,
                            JSObjectRef thisObject,
                            size_t argumentCount,
                            const JSValueRef arguments[],
                            JSValueRef *exception)
{
    (void)noused_context;
    (void)function;
    (void)thisObject;
    (void)argumentCount;
    (void)arguments;
    (void)exception;
    JSContextRef context = get_global_context();
    gboolean _has_fatal_error = FALSE;
    JSValueRef r = NULL;
    if (argumentCount != 0) {
        js_fill_exception(context, exception,
            "the quit except 0 paramters but passed in %d", argumentCount);
        return NULL;
    }
    
    if (!_has_fatal_error) {
        
        JSData* data = g_new0(JSData, 1);
        data->exception = exception;
        data->webview = get_global_webview();

         shutdown_quit (data);
        r = JSValueMakeNull(context);

        g_free(data);

    }
    
    if (_has_fatal_error)
        return NULL;
    else
        return r;
}

extern char *  shutdown_get_username(JSData*);
static JSValueRef __get_username__ (JSContextRef noused_context,
                            JSObjectRef function,
                            JSObjectRef thisObject,
                            size_t argumentCount,
                            const JSValueRef arguments[],
                            JSValueRef *exception)
{
    (void)noused_context;
    (void)function;
    (void)thisObject;
    (void)argumentCount;
    (void)arguments;
    (void)exception;
    JSContextRef context = get_global_context();
    gboolean _has_fatal_error = FALSE;
    JSValueRef r = NULL;
    if (argumentCount != 0) {
        js_fill_exception(context, exception,
            "the get_username except 0 paramters but passed in %d", argumentCount);
        return NULL;
    }
    
    if (!_has_fatal_error) {
        
        JSData* data = g_new0(JSData, 1);
        data->exception = exception;
        data->webview = get_global_webview();

        const char* c_return =  shutdown_get_username (data);
        
        if (c_return != NULL) {
            r = jsvalue_from_cstr(context, c_return);
        } else {
            _has_fatal_error = TRUE;
            js_fill_exception(context, exception, "the return string is NULL");
        }


        g_free(data);

    }
    
    if (_has_fatal_error)
        return NULL;
    else
        return r;
}


static const JSStaticFunction Shutdown_class_staticfuncs[] = {
    
{ "quit", __quit__, kJSPropertyAttributeReadOnly },

{ "get_username", __get_username__, kJSPropertyAttributeReadOnly },

    { NULL, NULL, 0}
};
static const JSClassDefinition Shutdown_class_def = {
    0,
    kJSClassAttributeNone,
    "ShutdownClass",
    NULL,
    NULL, //class_staticvalues,
    Shutdown_class_staticfuncs,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL,
    NULL
};

JSClassRef get_Shutdown_class()
{
    static JSClassRef _class = NULL;
    if (_class == NULL) {
        _class = JSClassCreate(&Shutdown_class_def);
    }
    return _class;
}
