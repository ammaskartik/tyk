// +build coprocess
// +build lua

package main

/*
#cgo pkg-config: luajit

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "coprocess/sds/sds.h"

#include "coprocess/api.h"

#include "coprocess/lua/binding.h"

#include <lua.h>
#include <lualib.h>
#include <lauxlib.h>

void LoadCachedMiddleware();

static void LuaInit() {
  // TODO: Cache the middlewares.
}

static void LuaReload() {
}

static void LoadMiddleware(char* middleware_file, char* middleware_contents) {
}

static void LoadMiddlewareIntoState(lua_State* L, char* middleware_name, char* middleware_contents) {
  luaL_dostring(L, middleware_contents);
}

static struct CoProcessMessage* LuaDispatchHook(struct CoProcessMessage* object) {

  struct CoProcessMessage* outputObject = malloc(sizeof *outputObject);

  lua_State *L = luaL_newstate();

  luaL_openlibs(L);
  luaL_dofile(L, "coprocess/lua/tyk/core.lua");

  LoadCachedMiddleware(L);

  lua_getglobal(L, "dispatch");
  lua_pushlstring(L, object->p_data, object->length);
  lua_pcall(L, 1, 1, 0);

  size_t lua_output_length = lua_tointeger(L, 0);
  const char* lua_output_data = lua_tolstring(L, 1, &lua_output_length);

  char* output = malloc(lua_output_length);
  memmove(output, lua_output_data, lua_output_length);

  lua_close(L);

  outputObject->p_data = (void*)output;
  outputObject->length = lua_output_length;

  return outputObject;
}

static void LuaDispatchEvent(char* event_json) {
}
*/
import "C"

import(
  "io/ioutil"
  "unsafe"
  "path"

  "github.com/TykTechnologies/tyk/coprocess"
  "github.com/Sirupsen/logrus"
)

// CoProcessName declares the driver name.
const CoProcessName string = "lua"

const MiddlewareBasePath = "middleware/lua"

// MessageType sets the default message type.
var MessageType = coprocess.JsonMessage

var gMiddlewareCache *map[string]string

// LuaDispatcher implements a coprocess.Dispatcher
type LuaDispatcher struct {
	coprocess.Dispatcher
  MiddlewareCache map[string]string
}

// Dispatch takes a CoProcessMessage and sends it to the CP.
func (d *LuaDispatcher) Dispatch(objectPtr unsafe.Pointer) unsafe.Pointer {
	var object *C.struct_CoProcessMessage
	object = (*C.struct_CoProcessMessage)(objectPtr)

	var newObjectPtr *C.struct_CoProcessMessage
	newObjectPtr = C.LuaDispatchHook(object)

	return unsafe.Pointer(newObjectPtr)
}

func (d *LuaDispatcher) Reload() {
  files, _ := ioutil.ReadDir("./middleware/lua")
  if d.MiddlewareCache == nil {
    d.MiddlewareCache = make(map[string]string, len(files))
    gMiddlewareCache = &d.MiddlewareCache
  } else {
    for k := range d.MiddlewareCache {
      delete(d.MiddlewareCache, k)
    }
  }

  for _, f := range files {
    middlewarePath := path.Join(MiddlewareBasePath, f.Name())
    contents, err := ioutil.ReadFile(middlewarePath)
    if err != nil {
      log.WithFields(logrus.Fields{
        "prefix": "coprocess",
      }).Error("Failed to read middleware file: ", err)
    }
    d.MiddlewareCache[f.Name()] = string(contents)
  }

  C.LuaReload()
}

//export LoadCachedMiddleware
func LoadCachedMiddleware(luaState unsafe.Pointer) {
  for middlewareName, middlewareContents := range *gMiddlewareCache {
    var cMiddlewareName, cMiddlewareContents *C.char
    cMiddlewareName = C.CString(middlewareName)
    cMiddlewareContents = C.CString(middlewareContents)
    C.LoadMiddlewareIntoState(luaState, cMiddlewareName, cMiddlewareContents);
    C.free(unsafe.Pointer(cMiddlewareName))
    C.free(unsafe.Pointer(cMiddlewareContents))
  }
  return
}

func (d *LuaDispatcher) DispatchEvent(eventJSON []byte) {
  var CEventJSON *C.char
  CEventJSON = C.CString(string(eventJSON))
  C.LuaDispatchEvent(CEventJSON)
  C.free(unsafe.Pointer(CEventJSON))
  return
}

func LuaInit() {
  C.LuaInit()
}

// NewCoProcessDispatcher wraps all the actions needed for this CP.
func NewCoProcessDispatcher() (dispatcher coprocess.Dispatcher, err error) {

  LuaInit()

  dispatcher, err = &LuaDispatcher{}, nil

  dispatcher.Reload()

	if err != nil {
		log.WithFields(logrus.Fields{
			"prefix": "coprocess",
		}).Error(err)
	}

	return dispatcher, err
}