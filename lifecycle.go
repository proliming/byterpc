package byterpc

type Callback func()

type LifeCycle interface {
	Start()

	GetDoneChan() chan struct{}

	Stop()

	OnStart(l Callback)

	OnStop(l Callback)

	notifyStart()

	notifyStop()
}

func NewLifeCycleCtl() LifeCycle {
	return &lifeCycleController{
		done: make(chan struct{}),
	}
}

type lifeCycleController struct {
	done             chan struct{} // Channel used only to indicate should shutdown
	onStartCallbacks []Callback
	onStopCallbacks  []Callback
}

func (c *lifeCycleController) GetDoneChan() chan struct{} {
	return c.done
}

func (c *lifeCycleController) Start() {
	c.notifyStart()
}

func (c *lifeCycleController) Stop() {
	ch := c.GetDoneChan()
	select {
	case <-ch:
	default:
		c.notifyStop()
		close(ch)
	}
}

func (c *lifeCycleController) OnStart(l Callback) {
	c.onStartCallbacks = append(c.onStartCallbacks, l)
}
func (c *lifeCycleController) OnStop(l Callback) {
	c.onStopCallbacks = append(c.onStopCallbacks, l)
}

func (c *lifeCycleController) notifyStart() {
	for _, f := range c.onStartCallbacks {
		f()
	}
}

func (c *lifeCycleController) notifyStop() {
	for _, f := range c.onStopCallbacks {
		f()
	}
}
