package tasksui

type Task interface {
	GetTitle() string
	SetTitle(title string)
	GetDescription() string
	SetDescription(description string)
	GetEstimate() float32
	SetEstimate(estimate float32)
	GetTfsTaskID() int
	SetTfsTaskID(taskID int)
	Clone() Task
}

type Table interface {
	GetTasks() []Task
	SetTask(tsk Task, index int)
}
