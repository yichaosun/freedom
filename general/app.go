package general

import (
	"fmt"
	"reflect"

	"github.com/go-redis/redis"
	"github.com/jinzhu/gorm"

	"github.com/kataras/iris/mvc"

	"github.com/8treenet/gcache"
	"github.com/kataras/iris"
	"github.com/kataras/iris/context"
)

// NewApplication .
func NewApplication() *Application {
	globalAppOnce.Do(func() {
		globalApp = new(Application)
		globalApp.IrisApp = iris.New()
		globalApp.pool = newServicePool()
		globalApp.rpool = newRepoPool()
		globalApp.comPool = newComponentPool()
		boots = make([]func(Initiator), 0)
		globalApp.IrisApp.Logger().SetTimeFormat("2006-01-02 15:04:05.000")
	})
	return globalApp
}

// Application .
type Application struct {
	IrisApp  *iris.Application
	pool     *ServicePool
	rpool    *RepositoryPool
	comPool  *ComponentPool
	Database struct {
		db      *gorm.DB
		cache   gcache.Plugin
		Install func() (db *gorm.DB, cache gcache.Plugin)
	}

	Redis struct {
		client  *redis.Client
		Install func() (client *redis.Client)
	}
	Middleware []context.Handler
	Prometheus *Prometheus
}

// CreateParty .
func (app *Application) CreateParty(relativePath string, handlers ...context.Handler) iris.Party {
	return app.IrisApp.Party(relativePath, handlers...)
}

// BindController .
func (app *Application) BindController(relativePath string, controller interface{}, service ...interface{}) {
	mvcApp := mvc.New(app.IrisApp.Party(relativePath))
	deps := append(app.generalDep(), service...)
	mvcApp.Register(deps...)
	mvcApp.Handle(controller)
	return
}

// BindControllerByParty .
func (app *Application) BindControllerByParty(party iris.Party, controller interface{}, service ...interface{}) {
	mvcApp := mvc.New(party)
	deps := append(app.generalDep(), service...)
	mvcApp.Register(deps...)
	mvcApp.Handle(controller)
	return
}

// GetService .
func (app *Application) GetService(ctx iris.Context, service interface{}) {
	app.pool.get(ctx.Values().Get(runtimeKey).(*appRuntime), service)
	return
}

// BindService .
func (app *Application) BindService(f interface{}) {
	outType, err := parsePoolFunc(f)
	if err != nil {
		panic(fmt.Sprintf("%v : %s", f, fmt.Sprint(err)))
	}
	app.pool.bind(outType, f)
}

// BindRepository .
func (app *Application) BindRepository(f interface{}) {
	outType, err := parsePoolFunc(f)
	if err != nil {
		panic(fmt.Sprintf("%v : %s", f, fmt.Sprint(err)))
	}
	app.rpool.bind(outType, f)
}

// BindComponent .
func (app *Application) BindComponent(single bool, com interface{}) {
	if !single {
		outType, err := parsePoolFunc(com)
		if err != nil {
			panic(fmt.Sprintf("%v : %s", com, fmt.Sprint(err)))
		}
		app.comPool.bind(single, outType, com)
		return
	}
	if reflect.TypeOf(com).Kind() != reflect.Ptr {
		panic("single:true, The component must be an object")
	}
	app.comPool.bind(single, reflect.TypeOf(com), com)
}

// GetComponent .
func (app *Application) GetComponent(ctx iris.Context, com interface{}) {
	app.comPool.get(ctx.Values().Get(runtimeKey).(*appRuntime), reflect.ValueOf(com).Elem())
}

// AsyncCachePreheat .
func (app *Application) AsyncCachePreheat(f func(repo *Repository)) {
	rb := new(Repository)
	go f(rb)
}

// CachePreheat .
func (app *Application) CachePreheat(f func(repo *Repository)) {
	rb := new(Repository)
	f(rb)
}

func (app *Application) generalDep() (result []interface{}) {
	result = append(result, func(ctx iris.Context) (rt Runtime) {
		rt = ctx.Values().Get(runtimeKey).(Runtime)
		return
	})
	return
}

// Run .
func (app *Application) Run(serve iris.Runner, irisConf iris.Configuration) {
	app.addMiddlewares(irisConf)
	if app.Database.Install != nil {
		app.Database.db, app.Database.cache = app.Database.Install()
	}

	if app.Redis.Install != nil {
		app.Redis.client = app.Redis.Install()
	}

	for index := 0; index < len(boots); index++ {
		boots[index](app)
	}

	repositoryAPIRun(irisConf)
	app.IrisApp.Run(serve, iris.WithConfiguration(irisConf))
}

func (app *Application) addMiddlewares(irisConf iris.Configuration) {
	app.IrisApp.Use(newRuntimeHandle())
	app.IrisApp.Use(globalApp.pool.freeHandle())
	if pladdr, ok := irisConf.Other["prometheus_listen_addr"]; ok {
		app.Prometheus = newPrometheus(irisConf.Other["service_name"].(string), pladdr.(string))
		globalApp.IrisApp.Use(newPrometheusHandle(app.Prometheus))
	}
	globalApp.IrisApp.Use(app.Middleware...)
	globalApp.IrisApp.Use(newRecover())
}