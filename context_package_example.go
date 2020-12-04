/*
The use of context in Golang is a highly debated topic. Some argue it should only be used to manage the lifetime of
go routines and api calls. Some use it as a single object to store data that can be used in other functions.

Google's Golang team explicitly describes how context should be used https://golang.org/pkg/context.
In the overview, the use of WithCancel, WithDeadline, and WithTimeout are discussed along with
some general best practices. However, context can also be used to store information.

Here is where developers can abuse context.

To store information in context, you use context.WithValue(context.Context, key, val interface{})
Notice that key and val are both of type interface{}, which in Golang means ANYTHING.
That means you lose all the benefits of static typing a compiled language affords you.
It's basically a bag of unknown keys and values that is not easy to maintain.
That's a big reason why developers dislike context.

However, it still has its place if used properly. The context package describes the usage of WithValue():

"WithValue returns a copy of parent in which the value associated with key is val."
"Use context Values only for request-scoped data that transits processes and APIs, not for passing optional parameters to functions."

The key here is "request-scoped data that transits processes and APIs." It is here where I developed a context library
to leverage the accessibility of context and provided all of our http handlers with the same common data.

This library is importable to any service that deals with context.
*/
package context

import (
    "context"
)

// context.WithValue() needs a key. I define a context key as a struct{} to avoid
//   possible allocations versus if i were to use a string because struct{} is a concrete type.
// this is a performance optimization.
type (
    mainContextKey struct{}
)

// here i store all the information every handler could use.
// this is a truncated example.
type mainContext struct {
    RequestID string
    IPAddress string
}

// mainContextKey and mainContext are not exportable because the first letter is not capitalized.
// this package is meant to be exportable but i don't want other packages accessing these values.
// using this strategy, it helps define exactly how this package should be used.
// the logic of this package is only exposed through getters and setters, which are defined below.

// every http request has its own context.
// in middleware, we populate mainContext with all the data needed and add it to
//   request's context so it's available to any handler (line 117 in http_handler_example.go).
// request := request.WithContext(SetMainContext(request.Context(), mainContext))
func SetMainContext(ctx context.Context, data mainContext) context.Context {
    return context.WithValue(ctx, mainContextKey{}, data)
}

// retrieve the data from context using the context key.
// HOWEVER, remember that values stored in the context are interface{}.
// so a type assertion is required.
func GetMainContext(ctx context.Context) mainContext {
    mainCtx, ok := ctx.Value(mainContextKey{}).(mainContext)
    if ok {
        return mainCtx
    }

    // if type assertion fails, return an empty mainContext.
    // this makes the job of the rest of the getters and setters easier.
    return mainContext{}
}

func SetRequestID(ctx context.Context, requestID string) context.Context {
    data := GetMainContext(ctx)
    data.RequestID = requestID
    return context.WithValue(ctx, mainContextKey{}, data)
}

func GetRequestID(ctx context.Context) string {
    data := GetMainContext(ctx)
    return data.RequestID
}

/*
The logic used in the Getters and Setters shows how I only deal with one object.
Since context.WithValue() returns a copy of the context, I want to avoid calling it multiple times.
So instead of doing this:

type (
    requestIDCtxKey struct{}
    ipAddressCtxKey struct{}
)

ctx = context.WithValue(request.Context(), requestIDCtxKey{}, requestID)
ctx = context.WithValue(ctx, ipAddressCtxKey{}, ipAddress)
etc...
etc...

request = request.WithContext(ctx)


I can simply do this:

data := mainContext{
    RequestID: requestID,
    IPAddress: ipAddress,
    etc...
}

request = request.WithContext(SetMainContext(request.Context(), data))

Request's context is now available to any handler (line 117 in http_handler_example.go)
*/
