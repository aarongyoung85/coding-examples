/*
This is an example of a basic http handler that I wrote for a web service.
For simplicity, organization is not prioritized here.
Usually, this file would be broken up into separate files/packages.
*/
package examplePackage

import (
    "context"
    "fmt"
    "net/http"
    "database/sql"
    errs "errors"

    "github.com/private-repo/negotiate"
    "github.com/private-repo/settings"
    "github.com/sirupsen/logrus"
    "gihub.com/husobee/vestigo"
    "github.com/pkg/errors"
)


// pre-defined sentinel errors to leverage Golang's error wrapping.
// the first letters are lower case because we don't want these to be exportable.
// we can only use them in files within the same package (examplePackage).
var (
    errBadRequest = errors.New("input error")
    errInternal = errors.New("internal error")
)

type Controller struct {
    settingsClient settings.Client
    settingsData userSettingsData
    DB *sql.DB
}

// these struct parameters have to be capitalized because we need to decode json.
type userSettingsData struct {
    Enabled bool `json:"enabled"`
    APIKey string `json:"api_key"`
}

func main() {
    // i instantiate a pointer when I create the variable here because there will be no 
    //   ambiguity in the usage of the variable "c" for the rest of this function
    c := &Controller{
        settingsClient: settings.NewClient(),
    }

    if err := c.InitializeUserSettings(); err != nil {
        panic(err)
    }

    router := vestigo.NewRouter()
    // i include versions in the routes from the start so versioning is easier to manage moving forward.
    router.Post("/v1/user", c.CreateUserHandler)
    router.Post("/v1/update-settings", c.UpdateUserSettingsHandler)

    // to demonstrate RESTful API design, i include these routes but the logic isn't provided here.
    router.Get("/v1/user/:user_id", c.GetUserHandler)
    router.Delete("/v1/user/:user_id", c.DeleteUserHandler)

    // for this route i would include query params in the logic to deal with pagination.
    // eg. /v1/users?limit=10&offset=5
    router.Get("/v1/users", c.GetAllUsersHandler)
}

// you'll notice that all method receivers are pointers (c *Controller).
// the convention in Golang is if a function requires a pointer method reciever, all method
//   receivers should be pointers to avoid confusion.
// i'll explain why these are pointers shortly
func (c *Controller) InitializeUserSettings() error {
    // notice here I don't instantiate the variable as a pointer like i did in main().
    usd := userSettingsData{}

    // but here, I explicitly pass a pointer to c.SettingsClient.Get 
    if err := c.settingsClient.Get(&usd); err != nil {
        // first example of using sentinel errors in Golang's error wrapping.
        return fmt.Errorf("failed to get user settings. %s. %w", err, errInternal)
    }

    // a lot of Golang code instantiates a pointer when the variable is created.
    // i prefer instantiating as a value and EXPLICITLY passing a pointer when needed.
    // i believe this pattern of programming encourages functional-style programming
    //   which is more explicit, easier to test, easier to maintain, and easier to read

    // because i modify the Controller struct here, i need the method receiver to be a pointer.
    // if the method receiver was a value, this line of code will only live for the life of this function.
    // i want every function that has the same receiver to have the modified data.
    c.settingsData = usd

	return nil
}

// POST /v1/update-settings
func (c *Controller) UpdateUserSettingsHandler(rw http.ResponseWriter, req *http.Request) {
    n := negotiate.GetNegotiator(req)

    // here, you truly see why the method receiver is a pointer.
    // because of this handler, i can update the service's settings whenever i want
    //   by simply curling the endpoint.
    // since the method receiver is a pointer, all functions will get the updated settings.
    if err := c.InitializeUserSettings(); err != nil {
        logrus.WithError(err).Error("failed to update user settings")
        n.Respond(rw, http.StatusInternalServerError, response.Error(nil))
        return
    }

    // return c.SettingsData to see what the updated settings are
    n.Respond(rw, http.StatusOK, response.Success(c.SettingsData))
}

// POST /v1/user
func (c *Controller) CreateUserHandler(rw http.ResponseWriter, req *http.Request) {
    // Golang's use of context is an area of much debate and i discuss it in context_package_example.go.
    ctx := req.Context()
    lf := logrus.Fields{"handler": "CreateUser"}
    n := negotiate.GetNegotiator(req)

    if !c.SettingsData.Enabled {
        // could argue this could return different statuses.
        n.Respond(rw, http.StatusNotImplemented, response.Error(nil))
        return
    }

    // i like the pattern of not putting all the logic in the main handler and using
    //   a function like this that separates the logic.
    // the reason is only the main handler knows how to respond to the client and all the 
    //   possible http statuses are nicely laid out here.
    // this makes the code easier to maintain because anyone can look at one handler and
    //   instantly understand what to expect.
    // i leverage Golang's error wrapping to communicate to the main handler what the status should be.
    userResp, err := c.handleCreateUser(ctx, req)
    lf["user_id"] = user.ID
    if err != nil {
        logrus.WithFields(lf).WithError(err).Error("failed to create user")

        // Golang's new (go1.13) way of dealing with errors.
        if errs.Is(err, errBadRequest) {
            // return the error so the client can fix it.
            n.Respond(rw, http.StatusBadRequest, response.Error(err))
        } else if errs.Is (err, errInternal){
            // don't want the client to know about internal errors.
            n.Respond(rw, http.StatusInternalServerError, response.Error(nil))
        }
        return
    }

    n.Respond(rw, http.StatusCreated, response.Success(userResp))
}

type createUserRequest struct {
    FullName string `json:"full_name"`
    Address string `json:"address"`
    City string `json:"city"`
    State string `json:"state"`
    ZipCode int `json:"zip_code"`
}

type createUserResponse struct {
    ID string `json:"id"`
}

// this function has all the logic and communicates to the main handler what it should return to the client.
func (c *Controller) handleCreateUser(ctx context.Context, req *http.Request) (createUserResponse, error) {
    // i instantiate the response to the function here so i can keep returning it without creating new literals.
    // i also instantiate it as a value, not a pointer.
    // returning pointers from a function in golang puts pressure on the garbage collector
    //   and decreases the performance of your application.
    // TLDR: returning pointers adds memory to the heap that the garbage collector has to track and clean up.
    // returning values keeps the memory on the stack along with the function, which is much more efficient.
    resp := createUserResponse{}

    cur := createUserRequest{}
    // again, explicitly declare a pointer when necessary (&cur).
    if err := json.NewDecoder(req.Body).Decode(&cur); err != nil {
        // use the %w directive and use a sentinel error, which gets interpreted to an http response code at the
        //   main handler level.
        // this function doesn't need to know about http response codes.
        return resp, fmt.Errorf("failed to decode. %s. %w", err, errBadRequest)
    }

    // this function doesn't modify "cur" so it doesn't need it to be a pointer.
    // ie. this function won't produce any side effects
    if err := validateCreateUserRequest(cur); err != nil {
        return resp, fmt.Errorf("failed to validate create user request. %s. %w", err, errBadRequest)
    }

    // didn't bother writing this function out.
    userID, err := c.DB.InsertUser(ctx, cur)
    if err != nil {
        // if something went wrong, it had to have been an internal server error level of error.
        return resp, fmt.Errorf("failed to insert user. %s. %w", err, errInternal)
    }

    resp.ID = userID
    return resp, nil
}

func validateCreateUserRequest(cur CreateUserRequest) error {
    // here, i'm saying "errs" is a slice of strings that has a length of 0 but a capacity of 5.
    // that means at this moment, "errs" is an empty slice, as you would expect.
    // BUT it can accept a maximum of 5 strings before it needs to allocate a new slice with greater capacity.
    // this is an optimization technique.
    errs := make([]string, 0, 5)

    // i could say the same thing using a literal: 
    // errs := []string{}

    // this creates a slice of strings with 0 length and 0 capacity.
    // when i want to append something, like i do below, there's no room to add another string
    //   so Golang will create a new slice with double the capacity (in this case, 1) in order to 
    //   fit the new data. if i keep appending, the capacity will double again to 2. if i add another,
    //   there'll be a new slice created with capacity of 4, and so on.
    // since i already know the maximum bound of the slice, i declare it when i make the slice.
    // this avoids extra allocations and improves performance.

    if cur.FullName == "" {
        errs = append(errs, "full name is required")
    }

    if cur.Address == "" {
        errs = append(errs, "address is required")
    }

    if cur.City == "" {
        errs = append(errs, "city is required")
    }

    if cur.State == "" || len(cur.State) != 2 {
        errs = append(errs, "state is required and must be 2 characters")
    }

    if cur.ZipCode == 0 {
        errs = append(errs, "zip code is required")
    }

    if len(errs) > 0 {
        return fmt.Errorf("%s", strings.Join(errs, "; "))
    }

    return nil
}
