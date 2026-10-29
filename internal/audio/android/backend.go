//go:build android

package android

/*
#cgo LDFLAGS: -llog

#include <jni.h>
#include <stdlib.h>
#include <string.h>

static JavaVM* g_vm = NULL;
static jclass g_audio = NULL;

static void registerAudio(JNIEnv* env) {
	if ((*env)->GetJavaVM(env, &g_vm) != 0) return;
	jclass local = (*env)->FindClass(env, "com/wails/app/AudioBridge");
	if (local == NULL) {
		(*env)->ExceptionClear(env);
		return;
	}
	g_audio = (jclass)(*env)->NewGlobalRef(env, local);
	(*env)->DeleteLocalRef(env, local);
}

static JNIEnv* audioEnv(int* detach) {
	*detach = 0;
	if (g_vm == NULL) return NULL;
	JNIEnv* env = NULL;
	jint r = (*g_vm)->GetEnv(g_vm, (void**)&env, JNI_VERSION_1_6);
	if (r == JNI_EDETACHED) {
		if ((*g_vm)->AttachCurrentThread(g_vm, &env, NULL) != 0) return NULL;
		*detach = 1;
	} else if (r != JNI_OK) {
		return NULL;
	}
	return env;
}

static void audioDetach(int detach) {
	if (detach && g_vm != NULL) (*g_vm)->DetachCurrentThread(g_vm);
}

static char* callString(const char* method) {
	int detach = 0;
	JNIEnv* env = audioEnv(&detach);
	if (env == NULL || g_audio == NULL) return NULL;
	jmethodID mid = (*env)->GetStaticMethodID(env, g_audio, method, "()Ljava/lang/String;");
	if (mid == NULL) {
		(*env)->ExceptionClear(env);
		audioDetach(detach);
		return NULL;
	}
	jstring js = (jstring)(*env)->CallStaticObjectMethod(env, g_audio, mid);
	if ((*env)->ExceptionCheck(env) || js == NULL) {
		(*env)->ExceptionClear(env);
		audioDetach(detach);
		return NULL;
	}
	const char* utf = (*env)->GetStringUTFChars(env, js, NULL);
	char* out = utf ? strdup(utf) : NULL;
	if (utf) (*env)->ReleaseStringUTFChars(env, js, utf);
	(*env)->DeleteLocalRef(env, js);
	audioDetach(detach);
	return out;
}

static void callVoid(const char* method) {
	int detach = 0;
	JNIEnv* env = audioEnv(&detach);
	if (env == NULL || g_audio == NULL) return;
	jmethodID mid = (*env)->GetStaticMethodID(env, g_audio, method, "()V");
	if (mid == NULL) {
		(*env)->ExceptionClear(env);
		audioDetach(detach);
		return;
	}
	(*env)->CallStaticVoidMethod(env, g_audio, mid);
	if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
	audioDetach(detach);
}

static void requestCapture(void) { callVoid("requestCapture"); }

static const char* takeUTF(JNIEnv* env, jstring value) {
	if (value == NULL) return NULL;
	return (*env)->GetStringUTFChars(env, value, NULL);
}

static void releaseUTF(JNIEnv* env, jstring value, const char* utf) {
	if (value != NULL && utf != NULL) (*env)->ReleaseStringUTFChars(env, value, utf);
}
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"

	"stereoscope/internal/audio"
)

var (
	leftBits  atomic.Uint32
	rightBits atomic.Uint32
	stateMu   sync.Mutex
	stateText = "idle"
)

//export Java_com_wails_app_AudioBridge_nativeRegister
func Java_com_wails_app_AudioBridge_nativeRegister(env *C.JNIEnv, clazz C.jclass) {
	C.registerAudio(env)
}

//export Java_com_wails_app_AudioBridge_nativeSetLevels
func Java_com_wails_app_AudioBridge_nativeSetLevels(env *C.JNIEnv, clazz C.jclass, left C.jfloat, right C.jfloat) {
	storeFloat(&leftBits, float32(left))
	storeFloat(&rightBits, float32(right))
}

//export Java_com_wails_app_AudioBridge_nativeSetCaptureState
func Java_com_wails_app_AudioBridge_nativeSetCaptureState(env *C.JNIEnv, clazz C.jclass, value C.jstring) {
	utf := C.takeUTF(env, value)
	if utf == nil {
		return
	}
	stateMu.Lock()
	stateText = C.GoString(utf)
	stateMu.Unlock()
	C.releaseUTF(env, value, utf)
}

func storeFloat(dst *atomic.Uint32, v float32) {
	dst.Store(*(*uint32)(unsafe.Pointer(&v)))
}

func loadFloat(src *atomic.Uint32) float32 {
	bits := src.Load()
	return *(*float32)(unsafe.Pointer(&bits))
}

type backend struct {
	changed chan struct{}
}

// Open returns the Android backend. The activity registers the Java bridge on start.
func Open() (audio.Backend, error) {
	return &backend{changed: make(chan struct{}, 1)}, nil
}

func (b *backend) Devices() ([]audio.Device, error) {
	raw := call("devicesJson")
	if raw == "" {
		return []audio.Device{{
			ID: "active", Name: "Active output", Default: true, Active: true, Channels: 2,
		}}, nil
	}
	var devices []audio.Device
	if err := json.Unmarshal([]byte(raw), &devices); err != nil {
		return nil, err
	}
	return devices, nil
}

func (b *backend) Select(string) error { return nil }

func (b *backend) Levels() ([]audio.PairLevels, error) {
	return []audio.PairLevels{{
		Pair:  audio.Pair{ID: "front", Label: "Front"},
		Left:  loadFloat(&leftBits),
		Right: loadFloat(&rightBits),
	}}, nil
}

func (b *backend) Sources() ([]audio.Source, error) {
	raw := call("sourcesJson")
	if raw == "" {
		return nil, nil
	}
	var sources []audio.Source
	if err := json.Unmarshal([]byte(raw), &sources); err != nil {
		return nil, err
	}
	return sources, nil
}

func (b *backend) PlayTest(string, string) error {
	return fmt.Errorf("test tones are not available on Android")
}

func (b *backend) Close() error { return nil }

func (b *backend) Changed() <-chan struct{} { return b.changed }

// CaptureState is the consent and capture status reported by Java.
func CaptureState() string {
	stateMu.Lock()
	defer stateMu.Unlock()
	return stateText
}

// BeginCapture asks the activity to show the media-projection consent prompt.
func BeginCapture() { C.requestCapture() }

func call(method string) string {
	cstr := C.CString(method)
	defer C.free(unsafe.Pointer(cstr))
	out := C.callString(cstr)
	if out == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(out))
	return C.GoString(out)
}
