<script setup>
defineProps({
  state: { type: Object, required: true },
  selectedBoard: { type: Object, default: null },
  taskFilterForm: { type: Object, required: true },
  uniqueAssignees: { type: Array, required: true },
  assigneeOptions: { type: Array, required: true },
  isScrumBoard: { type: Boolean, required: true },
  resetTaskFilters: { type: Function, required: true },
  isTaskFilterActive: { type: Boolean, required: true },
  tasksByColumn: { type: Object, required: true },
  dragState: { type: Object, required: true },
  onColumnDragOver: { type: Function, required: true },
  onColumnDragLeave: { type: Function, required: true },
  onColumnDrop: { type: Function, required: true },
  isTaskEditing: { type: Function, required: true },
  onTaskDragStart: { type: Function, required: true },
  onTaskDragEnd: { type: Function, required: true },
  onTaskDragOver: { type: Function, required: true },
  onTaskDragLeave: { type: Function, required: true },
  onTaskDrop: { type: Function, required: true },
  startTaskEdit: { type: Function, required: true },
  deleteTask: { type: Function, required: true },
  editTaskForm: { type: Object, required: true },
  saveTaskEdit: { type: Function, required: true },
  cancelTaskEdit: { type: Function, required: true },
  formatAssigneeLabel: { type: Function, required: true },
  isTaskAssignedToMe: { type: Function, required: true },
  formatTaskDueLabel: { type: Function, required: true },
  getTaskDueState: { type: Function, required: true },
  moveTargetByTaskId: { type: Object, required: true },
  moveTask: { type: Function, required: true },
  taskSprintTargetById: { type: Object, required: true },
  addTaskToSprint: { type: Function, required: true },
  removeTaskFromSprint: { type: Function, required: true },
  canManageSprint: { type: Boolean, required: true }
})

function formatSprintDate(value) {
  const text = String(value || '').trim()
  if (!text) return ''

  const match = text.match(/(\d{4})-(\d{2})-(\d{2})/)
  if (match) return `${match[3]}.${match[2]}.${match[1]}`

  const parsed = new Date(text)
  if (Number.isNaN(parsed.getTime())) return ''
  return parsed.toLocaleDateString('ru-RU')
}

function formatSprintDateRange(startsAt, endsAt) {
  const start = formatSprintDate(startsAt)
  const end = formatSprintDate(endsAt)

  if (start && end) return `Период: ${start} - ${end}`
  if (start) return `Период: с ${start}`
  if (end) return `Период: до ${end}`
  return 'Период: не задан'
}

function parseSprintDate(value) {
  const text = String(value || '').trim()
  if (!text) return null

  const match = text.match(/(\d{4})-(\d{2})-(\d{2})/)
  if (match) {
    const year = Number(match[1])
    const month = Number(match[2]) - 1
    const day = Number(match[3])
    const parsed = new Date(year, month, day)
    parsed.setHours(0, 0, 0, 0)
    return parsed
  }

  const parsed = new Date(text)
  if (Number.isNaN(parsed.getTime())) return null
  parsed.setHours(0, 0, 0, 0)
  return parsed
}

function getSprintHealth(sprint) {
  const status = String(sprint?.status || '').trim().toLowerCase()
  if (status === 'closed') return { label: 'Завершен', tone: 'closed' }

  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const endDate = parseSprintDate(sprint?.ends_at)

  if (endDate && endDate < today) return { label: 'Просрочен', tone: 'overdue' }
  if (status === 'active') return { label: 'В срок', tone: 'open' }
  if (status === 'planned') return { label: 'Запланирован', tone: 'planned' }
  return { label: 'В работе', tone: 'planned' }
}

function targetColumnsForTask(task, columns) {
  return (columns || []).filter((column) => column.id !== task?.column_id)
}

function targetSprintsForTask(task, sprints) {
  const currentSprintID = String(task?.sprint_id || '').trim()
  return (sprints || []).filter((sprint) => sprint.id !== currentSprintID)
}
</script>

<template>
  <section class="board-panel">
    <div v-if="state.error" class="error-banner">{{ state.error }}</div>

    <section class="filter-card" v-if="selectedBoard">
      <div v-if="isScrumBoard" class="sprint-switch-row">
        <label>Активный спринт</label>
        <select v-model="state.selectedSprintId">
          <option v-if="!state.sprints.length" value="" disabled>Спринтов пока нет</option>
          <option v-for="sprint in state.sprints" :key="sprint.id" :value="sprint.id">
            {{ sprint.name }}
          </option>
        </select>
      </div>

      <div class="filter-grid">
        <input v-model="taskFilterForm.query" placeholder="Поиск задач" />

        <select v-model="taskFilterForm.assignee">
          <option value="">Все исполнители</option>
          <option v-for="assignee in uniqueAssignees" :key="assignee" :value="assignee">
            {{ formatAssigneeLabel(assignee) }}
          </option>
        </select>

        <button
          class="ghost-btn"
          @click="resetTaskFilters"
          :disabled="!taskFilterForm.query && !taskFilterForm.assignee"
        >
          Сбросить фильтры
        </button>
      </div>
      <p v-if="isTaskFilterActive" class="hint-text">Перетаскивание отключено, пока активны фильтры.</p>
    </section>

    <section v-if="isScrumBoard && state.report" class="report-card">
      <h2>{{ state.report.sprint.name }} · Отчет</h2>
      <div class="report-meta">
        <p class="report-period">
          {{ formatSprintDateRange(state.report.sprint.starts_at, state.report.sprint.ends_at) }}
        </p>
        <span class="report-state" :class="`report-state--${getSprintHealth(state.report.sprint).tone}`">
          {{ getSprintHealth(state.report.sprint).label }}
        </span>
      </div>
      <div class="metrics-grid">
        <article>
          <strong>{{ state.report.total_tasks }}</strong>
          <span>Всего</span>
        </article>
        <article>
          <strong>{{ state.report.completed_tasks }}</strong>
          <span>Завершено</span>
        </article>
        <article>
          <strong>{{ state.report.remaining_tasks }}</strong>
          <span>Осталось</span>
        </article>
        <article>
          <strong>{{ state.report.velocity_tasks }}</strong>
          <span>Скорость</span>
        </article>
        <article>
          <strong>{{ state.report.completion_rate.toFixed(1) }}%</strong>
          <span>Выполнение</span>
        </article>
      </div>
    </section>

    <section class="board-lane-wrap" v-if="selectedBoard">
      <article
        v-for="column in state.columns"
        :key="column.id"
        class="column-card"
        :class="{ 'column-card--drop-target': dragState.taskId && dragState.overColumnId === column.id }"
        @dragover.prevent="onColumnDragOver(column.id, $event)"
        @dragleave="onColumnDragLeave(column.id, $event)"
        @drop.prevent="onColumnDrop(column.id)"
      >
        <header>
          <h3>{{ column.name }}</h3>
          <span>{{ (tasksByColumn[column.id] || []).length }}</span>
        </header>

        <div class="task-list">
          <div
            v-for="task in tasksByColumn[column.id] || []"
            :key="task.id"
            class="task-card"
            :class="{
              'task-card--dragging': dragState.taskId === task.id,
              'task-card--drop-target': dragState.overTaskId === task.id,
              'task-card--due-overdue': getTaskDueState(task.due_date) === 'overdue',
              'task-card--due-open': getTaskDueState(task.due_date) === 'open',
              'task-card--mine': isTaskAssignedToMe(task)
            }"
            :draggable="!state.loading && !isTaskEditing(task.id) && !isTaskFilterActive"
            @dragstart="onTaskDragStart(task, $event)"
            @dragend="onTaskDragEnd"
            @dragover.prevent="onTaskDragOver(task, $event)"
            @dragleave="onTaskDragLeave(task.id, $event)"
            @drop.stop.prevent="onTaskDrop(task)"
          >
            <div class="task-head">
              <h4>{{ isTaskEditing(task.id) ? 'Редактирование задачи' : task.title }}</h4>
            </div>

            <template v-if="isTaskEditing(task.id)">
              <div class="task-edit-grid">
                <input v-model="editTaskForm.title" placeholder="Название задачи" />
                <textarea v-model="editTaskForm.description" rows="3" placeholder="Описание" />
                <select v-model="editTaskForm.assignee">
                  <option value="">Не назначен</option>
                  <option
                    v-if="
                      editTaskForm.assignee &&
                      !assigneeOptions.some((member) => member.email === String(editTaskForm.assignee || '').trim().toLowerCase())
                    "
                    :value="editTaskForm.assignee"
                  >
                    {{ formatAssigneeLabel(editTaskForm.assignee) }}
                  </option>
                  <option v-for="member in assigneeOptions" :key="member.email" :value="member.email">
                    {{ member.label }}
                  </option>
                </select>
                <input v-model="editTaskForm.dueDate" type="date" />
                <select v-if="isScrumBoard" v-model="editTaskForm.sprintId" :disabled="!canManageSprint">
                  <option value="">Без спринта</option>
                  <option v-for="sprint in state.sprints" :key="sprint.id" :value="sprint.id">
                    {{ sprint.name }}
                  </option>
                </select>
                <p v-if="isScrumBoard" class="field-help">
                  Задача учитывается в отчете только с выбранным спринтом.
                  <span v-if="!canManageSprint"> Назначать и снимать спринт могут только владелец и наставник.</span>
                </p>
                <div class="task-edit-actions">
                  <button class="primary-btn" @click="saveTaskEdit(task.id)" :disabled="!editTaskForm.title.trim()">
                    Сохранить
                  </button>
                  <button class="ghost-btn" @click="cancelTaskEdit">Отмена</button>
                </div>
              </div>
            </template>
            <template v-else>
              <p v-if="task.description" class="task-desc">{{ task.description }}</p>
              <div class="meta-row">
                <span>{{ formatAssigneeLabel(task.assignee) }}</span>
                <span
                  class="due-badge"
                  :class="{
                    'due-badge--overdue': getTaskDueState(task.due_date) === 'overdue',
                    'due-badge--open': getTaskDueState(task.due_date) === 'open',
                    'due-badge--none': getTaskDueState(task.due_date) === 'none'
                  }"
                >
                  {{ formatTaskDueLabel(task.due_date) }}
                </span>
              </div>

              <div class="action-row task-action-select-row">
                <select
                  v-model="moveTargetByTaskId[task.id]"
                  :disabled="!targetColumnsForTask(task, state.columns).length"
                >
                  <option value="" disabled>
                    {{ targetColumnsForTask(task, state.columns).length ? 'Колонка' : 'Нет доступных колонок' }}
                  </option>
                  <option
                    v-for="targetColumn in targetColumnsForTask(task, state.columns)"
                    :key="targetColumn.id"
                    :value="targetColumn.id"
                  >
                    {{ targetColumn.name }}
                  </option>
                </select>
                <button class="ghost-btn" @click="moveTask(task.id)">Переместить</button>
              </div>

              <div class="action-row task-action-select-row" v-if="isScrumBoard">
                <select
                  v-model="taskSprintTargetById[task.id]"
                  :disabled="!canManageSprint || !targetSprintsForTask(task, state.sprints).length"
                >
                  <option value="" disabled>
                    {{ targetSprintsForTask(task, state.sprints).length ? 'Спринт' : 'Нет других спринтов' }}
                  </option>
                  <option
                    v-for="sprint in targetSprintsForTask(task, state.sprints)"
                    :key="sprint.id"
                    :value="sprint.id"
                  >
                    {{ sprint.name }}
                  </option>
                </select>
                <button class="ghost-btn" @click="addTaskToSprint(task.id)" :disabled="!canManageSprint">Назначить</button>
              </div>
              <p v-if="isScrumBoard" class="field-help">Назначьте спринт для учета задачи в отчете.</p>

              <div class="action-row task-sprint-row" v-if="isScrumBoard && task.sprint_id">
                <button class="ghost-btn task-backlog-btn" @click="removeTaskFromSprint(task.id)" :disabled="!canManageSprint">В бэклог</button>
                <span class="sprint-pill task-sprint-label">в спринте</span>
              </div>

              <div class="task-card-controls">
                <button class="ghost-btn" @click="startTaskEdit(task)">Изменить</button>
                <button class="danger-btn" @click="deleteTask(task.id)">Удалить</button>
              </div>
            </template>
          </div>
        </div>
      </article>
    </section>

    <section class="empty-state" v-else>
      <h2>Создайте доску для начала</h2>
      <p>Начните с цепочки рабочее пространство → проект → доска, после этого здесь появятся задачи.</p>
    </section>
  </section>
</template>
