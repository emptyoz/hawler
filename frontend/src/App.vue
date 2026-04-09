<script setup>
import { computed, onMounted, reactive, watch } from 'vue'
import AppLogo from './components/AppLogo.vue'
import BoardPanel from './components/BoardPanel.vue'
import ControlSidebar from './components/ControlSidebar.vue'
import ToastStack from './components/ToastStack.vue'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

const state = reactive({
  token: localStorage.getItem('hawler_token') || '',
  user: null,
  workspaces: [],
  workspaceMembers: [],
  projects: [],
  boards: [],
  columns: [],
  tasks: [],
  sprints: [],
  report: null,
  selectedWorkspaceId: '',
  selectedProjectId: '',
  selectedBoardId: '',
  selectedSprintId: '',
  loading: false,
  error: ''
})
const notifications = reactive([])

const authForm = reactive({
  mode: 'login',
  name: '',
  email: '',
  password: ''
})

const createWorkspaceForm = reactive({ name: '' })
const inviteMemberForm = reactive({ email: '', role: 'student' })
const createProjectForm = reactive({ name: '', description: '' })
const createBoardForm = reactive({ name: '', type: 'kanban' })
const createColumnForm = reactive({ name: '', kind: '', position: 0 })
const createTaskForm = reactive({ title: '', description: '', columnId: '', assignee: '', dueDate: '', sprintId: '' })
const createSprintForm = reactive({ name: '', goal: '', startsAt: '', endsAt: '' })
const taskFilterForm = reactive({ query: '', assignee: '', sprintId: '' })
const editTaskForm = reactive({ taskId: '', title: '', description: '', assignee: '', dueDate: '', sprintId: '' })

const moveTargetByTaskId = reactive({})
const taskSprintTargetById = reactive({})
const columnDraftById = reactive({})
const columnDeleteTargetById = reactive({})
const memberRoleDraftByUserId = reactive({})
const dragState = reactive({
  taskId: '',
  fromColumnId: '',
  overColumnId: '',
  overTaskId: ''
})

const hasToken = computed(() => Boolean(state.token))
const isScrumBoard = computed(() => selectedBoard.value?.type === 'scrum')
const canManageMembers = computed(() => selectedWorkspace.value?.role === 'owner')
const canManageBoard = computed(() => {
  const role = selectedWorkspace.value?.role || ''
  return role === 'owner' || role === 'mentor'
})
const canManageProject = computed(() => canManageBoard.value)
const canManageSprint = computed(() => canManageBoard.value)

const selectedWorkspace = computed(() => state.workspaces.find((w) => w.id === state.selectedWorkspaceId) || null)
const selectedProject = computed(() => state.projects.find((p) => p.id === state.selectedProjectId) || null)
const selectedBoard = computed(() => state.boards.find((b) => b.id === state.selectedBoardId) || null)
const selectedSprint = computed(() => state.sprints.find((s) => s.id === state.selectedSprintId) || null)
const workspaceMemberByEmail = computed(() => {
  const index = {}
  for (const member of state.workspaceMembers) {
    const email = String(member?.email || '').trim().toLowerCase()
    if (email && !index[email]) {
      index[email] = member
    }
  }
  return index
})
const assigneeOptions = computed(() => {
  return [...state.workspaceMembers]
    .sort((a, b) => {
      const left = `${String(a?.name || '').trim()} ${String(a?.email || '').trim()}`.toLowerCase()
      const right = `${String(b?.name || '').trim()} ${String(b?.email || '').trim()}`.toLowerCase()
      return left.localeCompare(right)
    })
    .map((member) => {
      const email = String(member?.email || '').trim().toLowerCase()
      const name = String(member?.name || '').trim()
      return {
        email,
        label: name ? `${name} · ${email}` : email
      }
    })
    .filter((option) => option.email)
})

const roleLabels = {
  owner: 'владелец',
  mentor: 'наставник',
  student: 'студент'
}

const boardTypeLabels = {
  kanban: 'Kanban',
  scrum: 'Scrum'
}

const sprintStatusLabels = {
  planned: 'запланирован',
  active: 'активен',
  closed: 'завершен'
}

const columnKindLabels = {
  backlog: 'бэклог',
  todo: 'к выполнению',
  in_progress: 'в работе',
  done: 'сделано',
  review: 'на ревью',
  qa: 'на тестировании'
}

function formatRoleLabel(role) {
  const key = String(role || '').trim().toLowerCase()
  return roleLabels[key] || key || 'не указана'
}

function formatBoardTypeLabel(boardType) {
  const key = String(boardType || '').trim().toLowerCase()
  return boardTypeLabels[key] || key || 'тип не указан'
}

function formatSprintStatusLabel(status) {
  const key = String(status || '').trim().toLowerCase()
  return sprintStatusLabels[key] || key || 'статус не указан'
}

function formatColumnKindLabel(kind) {
  const key = String(kind || '').trim().toLowerCase()
  return columnKindLabels[key] || key || 'тип не указан'
}

function normalizeAssigneeValue(value) {
  const raw = String(value || '').trim()
  if (!raw) return ''

  const lowered = raw.toLowerCase()
  if (workspaceMemberByEmail.value[lowered]) {
    return lowered
  }
  if (lowered.includes('@')) {
    return lowered
  }
  return raw
}

function formatAssigneeLabel(value) {
  const assignee = String(value || '').trim()
  if (!assignee) return 'не назначен'

  const lowered = assignee.toLowerCase()
  const member = workspaceMemberByEmail.value[lowered]
  if (member) {
    const name = String(member.name || '').trim()
    return name ? `${name} · ${lowered}` : lowered
  }
  return assignee
}

function isTaskAssignedToMe(task) {
  const assignee = String(task?.assignee || '').trim().toLowerCase()
  const currentUserEmail = String(state.user?.email || '').trim().toLowerCase()
  return Boolean(assignee) && Boolean(currentUserEmail) && assignee === currentUserEmail
}

const uniqueAssignees = computed(() => {
  const set = new Set()
  for (const task of state.tasks) {
    const assignee = String(task.assignee || '').trim()
    if (assignee) set.add(assignee)
  }
  return Array.from(set).sort((a, b) => a.localeCompare(b))
})
const filteredTasks = computed(() => {
  const query = taskFilterForm.query.trim().toLowerCase()
  const assignee = taskFilterForm.assignee.trim().toLowerCase()
  const sprintId = taskFilterForm.sprintId

  return state.tasks.filter((task) => {
    if (query) {
      const haystack = `${task.title || ''} ${task.description || ''}`.toLowerCase()
      if (!haystack.includes(query)) return false
    }

    if (assignee) {
      if (String(task.assignee || '').trim().toLowerCase() !== assignee) return false
    }

    if (isScrumBoard.value) {
      const taskSprintID = String(task.sprint_id || '').trim()
      const inBacklog = getColumnKind(task.column_id) === 'backlog'

      if (!sprintId) {
        return !taskSprintID && inBacklog
      }
      if (sprintId === 'none') {
        return !taskSprintID && inBacklog
      }

      if (taskSprintID === sprintId) return true
      if (!taskSprintID && inBacklog) return true
      return false
    }

    return true
  })
})
const isTaskFilterActive = computed(() => {
  if (taskFilterForm.query.trim()) return true
  if (taskFilterForm.assignee.trim()) return true
  return false
})
const allTaskCountByColumn = computed(() => {
  const grouped = {}
  for (const column of state.columns) grouped[column.id] = 0
  for (const task of state.tasks) {
    const columnId = task.column_id
    if (columnId && grouped[columnId] !== undefined) grouped[columnId] += 1
  }
  return grouped
})
const tasksByColumn = computed(() => {
  const grouped = {}
  for (const column of state.columns) grouped[column.id] = []
  for (const task of filteredTasks.value) {
    const columnId = task.column_id
    if (columnId && grouped[columnId]) grouped[columnId].push(task)
  }
  return grouped
})

function removeToast(toastID) {
  const index = notifications.findIndex((item) => item.id === toastID)
  if (index >= 0) notifications.splice(index, 1)
}

function pushToast(message, type = 'success', timeoutMs = 3200) {
  const text = String(message || '').trim()
  if (!text) return

  const id = `${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
  notifications.push({ id, message: text, type })

  if (timeoutMs > 0) {
    setTimeout(() => removeToast(id), timeoutMs)
  }
}

function setError(message, showToast = true) {
  state.error = message || 'Неизвестная ошибка'
  if (showToast) pushToast(state.error, 'error', 4200)
}

function clearError() {
  state.error = ''
}

function confirmAction(message) {
  if (typeof window === 'undefined') return true
  return window.confirm(message)
}

async function request(path, method = 'GET', body) {
  const headers = { 'Content-Type': 'application/json' }
  if (state.token) headers.Authorization = `Bearer ${state.token}`

  const response = await fetch(`${API_BASE_URL}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined
  })

  const text = await response.text()
  let payload = null
  if (text) {
    try {
      payload = JSON.parse(text)
    } catch {
      payload = { raw: text }
    }
  }

  if (!response.ok) {
    const message = payload?.error || payload?.raw || `${response.status} ${response.statusText}`
    throw new Error(message)
  }

  return payload
}

async function run(action) {
  clearError()
  state.loading = true
  try {
    await action()
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    setError(message)
  } finally {
    state.loading = false
  }
}

function persistToken(token) {
  state.token = token
  localStorage.setItem('hawler_token', token)
}

function clearSession() {
  state.token = ''
  state.user = null
  localStorage.removeItem('hawler_token')
}

async function fetchMe() {
  state.user = await request('/api/v1/auth/me')
}

async function submitAuth() {
  await run(async () => {
    const mode = authForm.mode
    const path = authForm.mode === 'register' ? '/api/v1/auth/register' : '/api/v1/auth/login'
    const body = authForm.mode === 'register'
      ? { name: authForm.name, email: authForm.email, password: authForm.password }
      : { email: authForm.email, password: authForm.password }

    const result = await request(path, 'POST', body)
    persistToken(result.token)
    await initializeData()
    pushToast(mode === 'register' ? 'Аккаунт создан' : 'Вход выполнен')
  })
}

async function initializeData() {
  await fetchMe()
  await loadWorkspaces()
}

async function loadWorkspaces() {
  state.workspaces = await request('/api/v1/workspaces')
  if (!state.selectedWorkspaceId && state.workspaces[0]) {
    state.selectedWorkspaceId = state.workspaces[0].id
  }
}

async function createWorkspace() {
  if (!createWorkspaceForm.name.trim()) return
  await run(async () => {
    await request('/api/v1/workspaces', 'POST', { name: createWorkspaceForm.name.trim() })
    createWorkspaceForm.name = ''
    await loadWorkspaces()
    pushToast('Рабочее пространство создано')
  })
}

function resetMemberRoleDrafts() {
  for (const userID of Object.keys(memberRoleDraftByUserId)) {
    delete memberRoleDraftByUserId[userID]
  }
}

function syncMemberRoleDrafts() {
  const known = new Set()
  for (const member of state.workspaceMembers) {
    known.add(member.user_id)
    memberRoleDraftByUserId[member.user_id] = member.role
  }
  for (const userID of Object.keys(memberRoleDraftByUserId)) {
    if (!known.has(userID)) {
      delete memberRoleDraftByUserId[userID]
    }
  }
}

async function loadWorkspaceMembers() {
  if (!state.selectedWorkspaceId) {
    state.workspaceMembers = []
    resetMemberRoleDrafts()
    return
  }
  state.workspaceMembers = await request(`/api/v1/workspaces/${state.selectedWorkspaceId}/members`)
  syncMemberRoleDrafts()
}

async function addWorkspaceMember() {
  if (!state.selectedWorkspaceId || !canManageMembers.value) return
  const email = inviteMemberForm.email.trim().toLowerCase()
  if (!email) return

  await run(async () => {
    await request(`/api/v1/workspaces/${state.selectedWorkspaceId}/members`, 'POST', {
      email,
      role: inviteMemberForm.role
    })
    inviteMemberForm.email = ''
    await loadWorkspaceMembers()
    pushToast('Участник добавлен')
  })
}

async function saveWorkspaceMemberRole(member) {
  if (!state.selectedWorkspaceId || !canManageMembers.value) return
  const role = memberRoleDraftByUserId[member.user_id]
  if (!role || role === member.role) return

  await run(async () => {
    await request(`/api/v1/workspaces/${state.selectedWorkspaceId}/members`, 'POST', {
      user_id: member.user_id,
      role
    })
    await loadWorkspaceMembers()
    pushToast('Роль участника обновлена')
  })
}

async function loadProjects() {
  if (!state.selectedWorkspaceId) {
    state.projects = []
    return
  }
  state.projects = await request(`/api/v1/projects?workspace_id=${state.selectedWorkspaceId}`)
  if (!state.projects.find((p) => p.id === state.selectedProjectId)) {
    state.selectedProjectId = state.projects[0]?.id || ''
  }
}

async function createProject() {
  if (!state.selectedWorkspaceId || !canManageProject.value || !createProjectForm.name.trim()) return
  await run(async () => {
    await request('/api/v1/projects', 'POST', {
      workspace_id: state.selectedWorkspaceId,
      name: createProjectForm.name.trim(),
      description: createProjectForm.description.trim()
    })
    createProjectForm.name = ''
    createProjectForm.description = ''
    await loadProjects()
    pushToast('Проект создан')
  })
}

async function loadBoards() {
  if (!state.selectedProjectId) {
    state.boards = []
    return
  }
  state.boards = await request(`/api/v1/boards?project_id=${state.selectedProjectId}`)
  if (!state.boards.find((b) => b.id === state.selectedBoardId)) {
    state.selectedBoardId = state.boards[0]?.id || ''
  }
}

async function createBoard() {
  if (!state.selectedProjectId || !canManageProject.value || !createBoardForm.name.trim()) return
  await run(async () => {
    await request('/api/v1/boards', 'POST', {
      project_id: state.selectedProjectId,
      name: createBoardForm.name.trim(),
      type: createBoardForm.type
    })
    createBoardForm.name = ''
    await loadBoards()
    pushToast('Доска создана')
  })
}

async function loadColumns() {
  if (!state.selectedBoardId) {
    state.columns = []
    resetColumnDrafts()
    return
  }
  state.columns = await request(`/api/v1/boards/${state.selectedBoardId}/columns`)
  syncColumnDrafts()
  if (!createTaskForm.columnId || !state.columns.some((c) => c.id === createTaskForm.columnId)) {
    createTaskForm.columnId = state.columns[0]?.id || ''
  }
}

function normalizeColumnKind(raw) {
  return String(raw || '').trim().toLowerCase().replaceAll('-', '_').replaceAll(' ', '_')
}

function resetColumnDrafts() {
  for (const columnID of Object.keys(columnDraftById)) delete columnDraftById[columnID]
  for (const columnID of Object.keys(columnDeleteTargetById)) delete columnDeleteTargetById[columnID]
}

function listDeleteTargetColumns(columnID) {
  return state.columns.filter((column) => column.id !== columnID)
}

function getDefaultDeleteTargetColumnID(columnID) {
  const backlogColumn = state.columns.find((column) => normalizeColumnKind(column.kind) === 'backlog' && column.id !== columnID)
  if (backlogColumn) return backlogColumn.id

  const todoColumn = state.columns.find((column) => normalizeColumnKind(column.kind) === 'todo' && column.id !== columnID)
  if (todoColumn) return todoColumn.id

  const fallbackColumn = listDeleteTargetColumns(columnID)[0]
  return fallbackColumn?.id || ''
}

function getEffectiveDeleteTargetColumnID(columnID) {
  const selected = columnDeleteTargetById[columnID] || ''
  const available = listDeleteTargetColumns(columnID)
  if (available.some((column) => column.id === selected)) {
    return selected
  }
  return getDefaultDeleteTargetColumnID(columnID)
}

function canDeleteBoardColumn(columnID) {
  const column = state.columns.find((item) => item.id === columnID)
  if (!column) return false
  if (normalizeColumnKind(column.kind) === 'backlog') return false
  return state.columns.length > 1
}

function syncColumnDrafts() {
  const known = new Set()
  for (const column of state.columns) {
    known.add(column.id)
    columnDraftById[column.id] = {
      name: column.name,
      kind: column.kind,
      position: column.position
    }
    columnDeleteTargetById[column.id] = getEffectiveDeleteTargetColumnID(column.id)
  }
  for (const columnID of Object.keys(columnDraftById)) {
    if (!known.has(columnID)) delete columnDraftById[columnID]
  }
  for (const columnID of Object.keys(columnDeleteTargetById)) {
    if (!known.has(columnID)) delete columnDeleteTargetById[columnID]
  }
}

async function createColumn() {
  if (!state.selectedBoardId || !canManageBoard.value || !createColumnForm.name.trim() || !createColumnForm.kind.trim()) return
  await run(async () => {
    await request(`/api/v1/boards/${state.selectedBoardId}/columns`, 'POST', {
      name: createColumnForm.name.trim(),
      kind: normalizeColumnKind(createColumnForm.kind),
      position: Number(createColumnForm.position || 0)
    })
    createColumnForm.name = ''
    createColumnForm.kind = ''
    createColumnForm.position = 0
    await loadColumns()
    pushToast('Колонка создана')
  })
}

async function updateBoardColumn(columnID) {
  if (!state.selectedBoardId || !canManageBoard.value) return
  const draft = columnDraftById[columnID]
  if (!draft) return

  const name = String(draft.name || '').trim()
  const kind = normalizeColumnKind(draft.kind)
  if (!name || !kind) return

  await run(async () => {
    await request(`/api/v1/boards/${state.selectedBoardId}/columns/${columnID}`, 'PATCH', {
      name,
      kind,
      position: Number(draft.position || 0)
    })
    await loadColumns()
    await loadTasks()
    if (isScrumBoard.value) await loadReport()
    pushToast('Колонка обновлена')
  })
}

async function deleteBoardColumn(columnID) {
  if (!state.selectedBoardId || !canManageBoard.value) return
  if (!canDeleteBoardColumn(columnID)) {
    setError('Колонку Backlog нельзя удалять')
    return
  }

  const total = allTaskCountByColumn.value[columnID] || 0
  const targetColumnID = getEffectiveDeleteTargetColumnID(columnID)
  const columnName = state.columns.find((item) => item.id === columnID)?.name || 'колонка'
  const targetColumnName = state.columns.find((item) => item.id === targetColumnID)?.name || 'К выполнению'
  if (total > 0 && !targetColumnID) {
    setError('Перед удалением нужна хотя бы одна колонка для переноса задач')
    return
  }
  const deleteConfirmMessage = total > 0
    ? `Удалить колонку "${columnName}" и перенести ${total} задач(и) в "${targetColumnName}"?`
    : `Удалить колонку "${columnName}"?`
  if (!confirmAction(deleteConfirmMessage)) return

  await run(async () => {
    let path = `/api/v1/boards/${state.selectedBoardId}/columns/${columnID}`
    if (targetColumnID) {
      path += `?target_column_id=${encodeURIComponent(targetColumnID)}`
    }
    await request(path, 'DELETE')
    await loadColumns()
    await loadTasks()
    if (isScrumBoard.value) await loadReport()
    pushToast('Колонка удалена')
  })
}

async function loadTasks() {
  if (!state.selectedBoardId) {
    state.tasks = []
    resetDragState()
    cancelTaskEdit()
    return
  }
  state.tasks = await request(`/api/v1/tasks?board_id=${state.selectedBoardId}`)
  if (editTaskForm.taskId && !state.tasks.some((task) => task.id === editTaskForm.taskId)) {
    cancelTaskEdit()
  }
}

async function createTask() {
  if (!state.selectedBoardId || !createTaskForm.title.trim()) return
  await run(async () => {
    const payload = {
      board_id: state.selectedBoardId,
      title: createTaskForm.title.trim(),
      description: createTaskForm.description.trim(),
      assignee: normalizeAssigneeValue(createTaskForm.assignee),
      due_date: createTaskForm.dueDate.trim()
    }
    if (isScrumBoard.value) {
      const sprintID = String(createTaskForm.sprintId || '').trim()
      if (sprintID) {
        payload.sprint_id = sprintID
        const todoColumn = state.columns.find((column) => String(column.kind || '').trim().toLowerCase() === 'todo')
        const fallbackColumn = state.columns.find((column) => String(column.kind || '').trim().toLowerCase() !== 'backlog')
        const targetColumn = todoColumn || fallbackColumn || state.columns[0]
        if (targetColumn?.id) {
          payload.column_id = targetColumn.id
        }
      }
    } else {
      payload.column_id = createTaskForm.columnId || undefined
    }

    await request('/api/v1/tasks', 'POST', payload)
    createTaskForm.title = ''
    createTaskForm.description = ''
    createTaskForm.sprintId = ''
    await loadTasks()
    if (isScrumBoard.value) await loadReport()
    pushToast('Задача создана')
  })
}

async function moveTask(taskId) {
  const targetColumnId = moveTargetByTaskId[taskId]
  if (!targetColumnId) return
  const task = state.tasks.find((item) => item.id === taskId)
  if (!task) return
  await run(async () => {
    const payload = {
      column_id: targetColumnId,
      position: 0
    }
    if (isScrumBoard.value && isBacklogColumn(targetColumnId)) {
      payload.sprint_id = ''
    } else {
      const moveSprintID = resolveMoveSprintID(task, targetColumnId)
      if (moveSprintID === false) return
      if (moveSprintID) payload.sprint_id = moveSprintID
    }

    const updatedTask = await request(`/api/v1/tasks/${taskId}/move`, 'PATCH', payload)
    await loadTasks()
    if (isScrumBoard.value) await loadReport()
    if (isScrumBoard.value && !updatedTask?.sprint_id) {
      taskSprintTargetById[taskId] = ''
    }
    moveTargetByTaskId[taskId] = ''
  })
}

function resetTaskFilters() {
  taskFilterForm.query = ''
  taskFilterForm.assignee = ''
  taskFilterForm.sprintId = isScrumBoard.value ? (state.selectedSprintId || '') : ''
}

function getColumnKind(columnID) {
  const column = state.columns.find((item) => item.id === columnID)
  return String(column?.kind || '').trim().toLowerCase()
}

function isBacklogColumn(columnID) {
  return getColumnKind(columnID) === 'backlog'
}

function resolveMoveSprintID(task, targetColumnID) {
  if (!isScrumBoard.value) return ''

  if (isBacklogColumn(targetColumnID)) return ''

  const currentSprintID = String(task.sprint_id || '').trim()
  if (currentSprintID) return currentSprintID

  const selectedSprintID = String(state.selectedSprintId || '').trim()
  if (selectedSprintID) return selectedSprintID

  setError('Выберите спринт для задачи перед переносом из бэклога')
  return false
}

function dueDateKey(raw) {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return ''
  return date.toISOString().slice(0, 10)
}

function getTaskDueState(raw) {
  const due = dueDateKey(raw)
  if (!due) return 'none'
  const today = new Date().toISOString().slice(0, 10)
  return due < today ? 'overdue' : 'open'
}

function formatTaskDueLabel(raw) {
  const due = dueDateKey(raw)
  if (!due) return 'Без срока'
  return due < new Date().toISOString().slice(0, 10)
    ? `Просрочено: ${due}`
    : `Срок: ${due}`
}

function asDateInputValue(raw) {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return ''
  return date.toISOString().slice(0, 10)
}

function isTaskEditing(taskId) {
  return editTaskForm.taskId === taskId
}

function startTaskEdit(task) {
  editTaskForm.taskId = task.id
  editTaskForm.title = task.title || ''
  editTaskForm.description = task.description || ''
  editTaskForm.assignee = normalizeAssigneeValue(task.assignee || '')
  editTaskForm.dueDate = asDateInputValue(task.due_date)
  editTaskForm.sprintId = task.sprint_id || ''
}

function cancelTaskEdit() {
  editTaskForm.taskId = ''
  editTaskForm.title = ''
  editTaskForm.description = ''
  editTaskForm.assignee = ''
  editTaskForm.dueDate = ''
  editTaskForm.sprintId = ''
}

async function saveTaskEdit(taskId) {
  if (editTaskForm.taskId !== taskId) return
  const title = editTaskForm.title.trim()
  if (!title) return
  const sourceTask = state.tasks.find((item) => item.id === taskId)
  const sourceSprintID = String(sourceTask?.sprint_id || '').trim()
  const targetSprintID = isScrumBoard.value && canManageSprint.value
    ? String(editTaskForm.sprintId || '').trim()
    : sourceSprintID

  await run(async () => {
    if (isScrumBoard.value && sourceSprintID !== targetSprintID) {
      if (targetSprintID) {
        await request(`/api/v1/sprints/${targetSprintID}/tasks/${taskId}`, 'POST')
      } else if (sourceSprintID) {
        await request(`/api/v1/sprints/${sourceSprintID}/tasks/${taskId}`, 'DELETE')
      }
    }

    const body = {
      title,
      description: editTaskForm.description.trim(),
      assignee: normalizeAssigneeValue(editTaskForm.assignee),
      due_date: editTaskForm.dueDate
    }

    await request(`/api/v1/tasks/${taskId}`, 'PATCH', body)
    await loadTasks()
    if (isScrumBoard.value) await loadReport()
    cancelTaskEdit()
    pushToast('Задача обновлена')
  })
}

function resetDragState() {
  dragState.taskId = ''
  dragState.fromColumnId = ''
  dragState.overColumnId = ''
  dragState.overTaskId = ''
}

function onTaskDragStart(task, event) {
  if (isTaskFilterActive.value) return
  if (!task?.id || !task?.column_id) return
  dragState.taskId = task.id
  dragState.fromColumnId = task.column_id
  dragState.overColumnId = task.column_id
  dragState.overTaskId = ''
  if (event?.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', task.id)
  }
}

function onTaskDragEnd() {
  resetDragState()
}

function onColumnDragOver(columnId, event) {
  if (isTaskFilterActive.value) return
  if (!dragState.taskId) return
  dragState.overColumnId = columnId
  dragState.overTaskId = ''
  if (event?.dataTransfer) {
    event.dataTransfer.dropEffect = 'move'
  }
}

function onColumnDragLeave(columnId, event) {
  if (dragState.overColumnId === columnId) {
    const nextTarget = event?.relatedTarget
    if (nextTarget && event?.currentTarget?.contains(nextTarget)) return
    dragState.overColumnId = ''
  }
}

function onTaskDragOver(task, event) {
  if (isTaskFilterActive.value) return
  if (!dragState.taskId || dragState.taskId === task.id || !task.column_id) return
  dragState.overColumnId = task.column_id
  dragState.overTaskId = task.id
  if (event?.dataTransfer) {
    event.dataTransfer.dropEffect = 'move'
  }
}

function onTaskDragLeave(taskId, event) {
  if (dragState.overTaskId === taskId) {
    const nextTarget = event?.relatedTarget
    if (nextTarget && event?.currentTarget?.contains(nextTarget)) return
    dragState.overTaskId = ''
  }
}

async function moveTaskWithPosition(taskID, columnID, position) {
  await run(async () => {
    const task = state.tasks.find((item) => item.id === taskID)
    if (!task) return

    const payload = {
      column_id: columnID,
      position
    }
    if (isScrumBoard.value && isBacklogColumn(columnID)) {
      payload.sprint_id = ''
    } else {
      const moveSprintID = resolveMoveSprintID(task, columnID)
      if (moveSprintID === false) return
      if (moveSprintID) payload.sprint_id = moveSprintID
    }

    const updatedTask = await request(`/api/v1/tasks/${taskID}/move`, 'PATCH', payload)
    await loadTasks()
    if (isScrumBoard.value) await loadReport()
    if (isScrumBoard.value && !updatedTask?.sprint_id) {
      taskSprintTargetById[taskID] = ''
    }
  })
}

async function onColumnDrop(columnId) {
  if (isTaskFilterActive.value) return
  if (!dragState.taskId || !columnId) return

  const draggedTask = state.tasks.find((task) => task.id === dragState.taskId)
  if (!draggedTask) {
    resetDragState()
    return
  }

  const laneTasks = tasksByColumn.value[columnId] || []
  let targetPosition = laneTasks.length
  if (draggedTask.column_id === columnId) {
    targetPosition = Math.max(0, laneTasks.length - 1)
  }

  try {
    await moveTaskWithPosition(draggedTask.id, columnId, targetPosition)
  } finally {
    resetDragState()
  }
}

async function onTaskDrop(targetTask) {
  if (isTaskFilterActive.value) return
  if (!dragState.taskId || !targetTask?.id || !targetTask?.column_id) return
  if (dragState.taskId === targetTask.id) {
    resetDragState()
    return
  }

  const draggedTask = state.tasks.find((task) => task.id === dragState.taskId)
  if (!draggedTask || !draggedTask.column_id) {
    resetDragState()
    return
  }

  let targetPosition = targetTask.position
  if (draggedTask.column_id === targetTask.column_id && draggedTask.position < targetTask.position) {
    targetPosition--
  }
  if (targetPosition < 0) {
    targetPosition = 0
  }

  try {
    await moveTaskWithPosition(draggedTask.id, targetTask.column_id, targetPosition)
  } finally {
    resetDragState()
  }
}

async function deleteTask(taskId) {
  const taskTitle = state.tasks.find((task) => task.id === taskId)?.title || 'эту задачу'
  if (!confirmAction(`Удалить "${taskTitle}"?`)) return

  await run(async () => {
    await request(`/api/v1/tasks/${taskId}`, 'DELETE')
    await loadTasks()
    pushToast('Задача удалена')
  })
}

async function loadSprints() {
  if (!state.selectedBoardId || !isScrumBoard.value) {
    state.sprints = []
    state.selectedSprintId = ''
    state.report = null
    return
  }
  state.sprints = await request(`/api/v1/sprints?board_id=${state.selectedBoardId}`)
  if (!state.sprints.find((s) => s.id === state.selectedSprintId)) {
    state.selectedSprintId = state.sprints[0]?.id || ''
  }
}

async function createSprint() {
  if (!state.selectedBoardId || !canManageSprint.value || !createSprintForm.name.trim()) return
  await run(async () => {
    await request('/api/v1/sprints', 'POST', {
      board_id: state.selectedBoardId,
      name: createSprintForm.name.trim(),
      goal: createSprintForm.goal.trim(),
      starts_at: createSprintForm.startsAt,
      ends_at: createSprintForm.endsAt,
      status: 'planned'
    })
    createSprintForm.name = ''
    createSprintForm.goal = ''
    createSprintForm.startsAt = ''
    createSprintForm.endsAt = ''
    await loadSprints()
    pushToast('Спринт создан')
  })
}

async function startSprint() {
  if (!canManageSprint.value || !state.selectedSprintId) return
  await run(async () => {
    await request(`/api/v1/sprints/${state.selectedSprintId}/start`, 'POST')
    await loadSprints()
    await loadReport()
    pushToast('Спринт запущен')
  })
}

async function closeSprint() {
  if (!canManageSprint.value || !state.selectedSprintId) return
  await run(async () => {
    const result = await request(`/api/v1/sprints/${state.selectedSprintId}/close`, 'POST')
    await loadTasks()
    await loadSprints()
    await loadReport()
    pushToast(`Спринт закрыт, в бэклог перенесено задач: ${result?.moved_tasks ?? 0}`)
  })
}

async function loadReport() {
  if (!state.selectedSprintId) {
    state.report = null
    return
  }
  state.report = await request(`/api/v1/sprints/${state.selectedSprintId}/report`)
}

async function addTaskToSprint(taskId) {
  if (!canManageSprint.value) return
  const sprintId = taskSprintTargetById[taskId] || state.selectedSprintId
  if (!sprintId) return
  const task = state.tasks.find((item) => item.id === taskId)
  const sourceSprintId = task?.sprint_id || ''
  if (task?.sprint_id === sprintId) {
    setError('Задача уже находится в этом спринте')
    return
  }
  await run(async () => {
    await request(`/api/v1/sprints/${sprintId}/tasks/${taskId}`, 'POST')
    await loadTasks()
    if (!state.selectedSprintId) {
      state.selectedSprintId = sprintId
    }
    await loadReport()
    if (sourceSprintId) {
      pushToast('Задача перенесена в другой спринт')
    } else {
      pushToast('Задача добавлена в спринт')
    }
  })
}

async function removeTaskFromSprint(taskId) {
  if (!canManageSprint.value) return
  const task = state.tasks.find((item) => item.id === taskId)
  if (!task?.sprint_id) return
  await run(async () => {
    await request(`/api/v1/sprints/${task.sprint_id}/tasks/${task.id}`, 'DELETE')
    await loadTasks()
    await loadReport()
    taskSprintTargetById[taskId] = ''
    pushToast('Задача перенесена в бэклог')
  })
}

function logout() {
  clearSession()
  state.workspaces = []
  state.workspaceMembers = []
  state.projects = []
  state.boards = []
  state.columns = []
  state.tasks = []
  state.sprints = []
  state.report = null
  state.selectedWorkspaceId = ''
  state.selectedProjectId = ''
  state.selectedBoardId = ''
  state.selectedSprintId = ''
  inviteMemberForm.email = ''
  inviteMemberForm.role = 'student'
  resetTaskFilters()
  cancelTaskEdit()
  resetColumnDrafts()
  resetMemberRoleDrafts()
  pushToast('Вы вышли из системы')
}

watch(
  () => state.selectedWorkspaceId,
  async () => {
    await run(async () => {
      await loadWorkspaceMembers()
      await loadProjects()
      await loadBoards()
      await loadColumns()
      await loadTasks()
      await loadSprints()
      await loadReport()
    })
  }
)

watch(
  () => state.selectedProjectId,
  async () => {
    await run(async () => {
      await loadBoards()
      await loadColumns()
      await loadTasks()
      await loadSprints()
      await loadReport()
    })
  }
)

watch(
  () => state.selectedBoardId,
  async () => {
    resetTaskFilters()
    cancelTaskEdit()
    await run(async () => {
      await loadColumns()
      await loadTasks()
      await loadSprints()
      await loadReport()
    })
  }
)

watch(
  () => state.selectedSprintId,
  async () => {
    if (isScrumBoard.value) {
      taskFilterForm.sprintId = state.selectedSprintId || ''
    }
    await run(async () => {
      await loadReport()
    })
  }
)

onMounted(async () => {
  if (!hasToken.value) return
  await run(async () => {
    await initializeData()
  })
})
</script>

<template>
  <div class="page-shell">
    <div class="aurora aurora-left" />
    <div class="aurora aurora-right" />
    <ToastStack :notifications="notifications" :remove-toast="removeToast" />

    <header class="app-header">
      <div class="brand-wrap">
        <div class="brand-badge">
          <AppLogo />
        </div>
        <div>
          <h1>Hawler</h1>
          <p>Платформа для студенческих команд</p>
        </div>
      </div>

      <div class="header-status">
        <span v-if="state.loading" class="status-chip">Синхронизация...</span>
        <span v-if="state.user" class="status-chip user-chip">{{ state.user.name }} · {{ state.user.email }}</span>
        <button v-if="state.user" class="ghost-btn" @click="logout">Выйти</button>
      </div>
    </header>

    <main class="app-grid">
      <ControlSidebar
        :state="state"
        :auth-form="authForm"
        :create-workspace-form="createWorkspaceForm"
        :invite-member-form="inviteMemberForm"
        :create-project-form="createProjectForm"
        :create-board-form="createBoardForm"
        :create-column-form="createColumnForm"
        :create-task-form="createTaskForm"
        :create-sprint-form="createSprintForm"
        :assignee-options="assigneeOptions"
        :member-role-draft-by-user-id="memberRoleDraftByUserId"
        :column-draft-by-id="columnDraftById"
        :column-delete-target-by-id="columnDeleteTargetById"
        :all-task-count-by-column="allTaskCountByColumn"
        :selected-workspace="selectedWorkspace"
        :selected-project="selectedProject"
        :selected-board="selectedBoard"
        :selected-sprint="selectedSprint"
        :can-manage-members="canManageMembers"
        :can-manage-project="canManageProject"
        :can-manage-board="canManageBoard"
        :can-manage-sprint="canManageSprint"
        :is-scrum-board="isScrumBoard"
        :format-role-label="formatRoleLabel"
        :format-board-type-label="formatBoardTypeLabel"
        :format-sprint-status-label="formatSprintStatusLabel"
        :format-column-kind-label="formatColumnKindLabel"
        :submit-auth="submitAuth"
        :create-workspace="createWorkspace"
        :save-workspace-member-role="saveWorkspaceMemberRole"
        :add-workspace-member="addWorkspaceMember"
        :create-project="createProject"
        :create-board="createBoard"
        :create-column="createColumn"
        :update-board-column="updateBoardColumn"
        :delete-board-column="deleteBoardColumn"
        :can-delete-board-column="canDeleteBoardColumn"
        :create-task="createTask"
        :create-sprint="createSprint"
        :start-sprint="startSprint"
        :close-sprint="closeSprint"
      />

      <BoardPanel
        :state="state"
        :selected-board="selectedBoard"
        :task-filter-form="taskFilterForm"
        :unique-assignees="uniqueAssignees"
        :assignee-options="assigneeOptions"
        :is-scrum-board="isScrumBoard"
        :reset-task-filters="resetTaskFilters"
        :is-task-filter-active="isTaskFilterActive"
        :tasks-by-column="tasksByColumn"
        :drag-state="dragState"
        :on-column-drag-over="onColumnDragOver"
        :on-column-drag-leave="onColumnDragLeave"
        :on-column-drop="onColumnDrop"
        :is-task-editing="isTaskEditing"
        :on-task-drag-start="onTaskDragStart"
        :on-task-drag-end="onTaskDragEnd"
        :on-task-drag-over="onTaskDragOver"
        :on-task-drag-leave="onTaskDragLeave"
        :on-task-drop="onTaskDrop"
        :start-task-edit="startTaskEdit"
        :delete-task="deleteTask"
        :edit-task-form="editTaskForm"
        :save-task-edit="saveTaskEdit"
        :cancel-task-edit="cancelTaskEdit"
        :format-assignee-label="formatAssigneeLabel"
        :is-task-assigned-to-me="isTaskAssignedToMe"
        :format-task-due-label="formatTaskDueLabel"
        :get-task-due-state="getTaskDueState"
        :move-target-by-task-id="moveTargetByTaskId"
        :move-task="moveTask"
        :task-sprint-target-by-id="taskSprintTargetById"
        :add-task-to-sprint="addTaskToSprint"
        :remove-task-from-sprint="removeTaskFromSprint"
        :can-manage-sprint="canManageSprint"
      />
    </main>
  </div>
</template>
