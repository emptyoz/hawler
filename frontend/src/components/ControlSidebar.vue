<script setup>
defineProps({
  state: { type: Object, required: true },
  authForm: { type: Object, required: true },
  createWorkspaceForm: { type: Object, required: true },
  inviteMemberForm: { type: Object, required: true },
  createProjectForm: { type: Object, required: true },
  createBoardForm: { type: Object, required: true },
  createColumnForm: { type: Object, required: true },
  createTaskForm: { type: Object, required: true },
  createSprintForm: { type: Object, required: true },
  assigneeOptions: { type: Array, required: true },
  memberRoleDraftByUserId: { type: Object, required: true },
  columnDraftById: { type: Object, required: true },
  columnDeleteTargetById: { type: Object, required: true },
  allTaskCountByColumn: { type: Object, required: true },
  selectedWorkspace: { type: Object, default: null },
  selectedProject: { type: Object, default: null },
  selectedBoard: { type: Object, default: null },
  selectedSprint: { type: Object, default: null },
  canManageMembers: { type: Boolean, required: true },
  canManageProject: { type: Boolean, required: true },
  canManageBoard: { type: Boolean, required: true },
  canManageSprint: { type: Boolean, required: true },
  isScrumBoard: { type: Boolean, required: true },
  formatRoleLabel: { type: Function, required: true },
  formatBoardTypeLabel: { type: Function, required: true },
  formatSprintStatusLabel: { type: Function, required: true },
  formatColumnKindLabel: { type: Function, required: true },
  submitAuth: { type: Function, required: true },
  createWorkspace: { type: Function, required: true },
  saveWorkspaceMemberRole: { type: Function, required: true },
  addWorkspaceMember: { type: Function, required: true },
  createProject: { type: Function, required: true },
  createBoard: { type: Function, required: true },
  createColumn: { type: Function, required: true },
  updateBoardColumn: { type: Function, required: true },
  deleteBoardColumn: { type: Function, required: true },
  canDeleteBoardColumn: { type: Function, required: true },
  createTask: { type: Function, required: true },
  createSprint: { type: Function, required: true },
  startSprint: { type: Function, required: true },
  closeSprint: { type: Function, required: true }
})
</script>

<template>
  <aside class="control-panel">
    <section class="card" v-if="!state.user">
      <h2>{{ authForm.mode === 'register' ? 'Создать аккаунт' : 'Вход' }}</h2>
      <div class="form-grid">
        <input v-if="authForm.mode === 'register'" v-model="authForm.name" placeholder="Имя" />
        <input v-model="authForm.email" placeholder="Электронная почта" type="email" />
        <input v-model="authForm.password" placeholder="Пароль" type="password" />
        <button class="primary-btn" @click="submitAuth">
          {{ authForm.mode === 'register' ? 'Зарегистрироваться' : 'Войти' }}
        </button>
        <button class="ghost-btn" @click="authForm.mode = authForm.mode === 'register' ? 'login' : 'register'">
          {{ authForm.mode === 'register' ? 'Переключиться на вход' : 'Переключиться на регистрацию' }}
        </button>
      </div>
    </section>

    <template v-else>
      <section class="card">
        <h2>Рабочее пространство</h2>
        <div class="stack">
          <select v-model="state.selectedWorkspaceId">
            <option value="" disabled>Выберите рабочее пространство</option>
            <option v-for="workspace in state.workspaces" :key="workspace.id" :value="workspace.id">
              {{ workspace.name }} ({{ formatRoleLabel(workspace.role) }})
            </option>
          </select>
          <div class="inline-form">
            <input v-model="createWorkspaceForm.name" placeholder="Название пространства" />
            <button class="primary-btn" @click="createWorkspace">Создать</button>
          </div>
        </div>
      </section>

      <section class="card" v-if="selectedWorkspace">
        <h2>Участники</h2>
        <div class="stack">
          <div class="member-list" v-if="state.workspaceMembers.length">
            <article class="member-row" v-for="member in state.workspaceMembers" :key="member.user_id">
              <div class="member-meta">
                <strong>{{ member.name }}</strong>
                <span>{{ member.email }}</span>
              </div>
              <div class="member-actions">
                <select v-model="memberRoleDraftByUserId[member.user_id]" :disabled="!canManageMembers">
                  <option value="owner">{{ formatRoleLabel('owner') }}</option>
                  <option value="mentor">{{ formatRoleLabel('mentor') }}</option>
                  <option value="student">{{ formatRoleLabel('student') }}</option>
                </select>
                <button
                  class="ghost-btn"
                  @click="saveWorkspaceMemberRole(member)"
                  :disabled="!canManageMembers || memberRoleDraftByUserId[member.user_id] === member.role"
                >
                  Сохранить
                </button>
              </div>
            </article>
          </div>
          <p v-else class="hint-text">Пока нет участников.</p>

          <template v-if="canManageMembers">
            <input v-model="inviteMemberForm.email" type="email" placeholder="Email участника" />
            <select v-model="inviteMemberForm.role">
              <option value="student">{{ formatRoleLabel('student') }}</option>
              <option value="mentor">{{ formatRoleLabel('mentor') }}</option>
              <option value="owner">{{ formatRoleLabel('owner') }}</option>
            </select>
            <button class="primary-btn" @click="addWorkspaceMember">Добавить участника</button>
          </template>
          <p v-else class="hint-text">Только владелец рабочего пространства может добавлять участников и менять роли.</p>
        </div>
      </section>

      <section class="card" v-if="selectedWorkspace">
        <h2>Проект</h2>
        <div class="stack">
          <select v-model="state.selectedProjectId">
            <option value="" disabled>Выберите проект</option>
            <option v-for="project in state.projects" :key="project.id" :value="project.id">
              {{ project.name }}
            </option>
          </select>
          <input v-model="createProjectForm.name" placeholder="Название проекта" :disabled="!canManageProject" />
          <textarea
            v-model="createProjectForm.description"
            rows="2"
            placeholder="Описание проекта"
            :disabled="!canManageProject"
          />
          <button class="primary-btn" @click="createProject" :disabled="!canManageProject">Создать проект</button>
          <p class="hint-text" v-if="!canManageProject">Только владелец или наставник может создавать проекты.</p>
        </div>
      </section>

      <section class="card" v-if="selectedProject">
        <h2>Доска</h2>
        <div class="stack">
          <select v-model="state.selectedBoardId">
            <option value="" disabled>Выберите доску</option>
            <option v-for="board in state.boards" :key="board.id" :value="board.id">
              {{ board.name }} · {{ formatBoardTypeLabel(board.type) }}
            </option>
          </select>
          <input v-model="createBoardForm.name" placeholder="Название доски" :disabled="!canManageProject" />
          <select v-model="createBoardForm.type" :disabled="!canManageProject">
            <option value="kanban">Kanban</option>
            <option value="scrum">Scrum</option>
          </select>
          <button class="primary-btn" @click="createBoard" :disabled="!canManageProject">Создать доску</button>
          <p class="hint-text" v-if="!canManageProject">Только владелец или наставник может создавать доски.</p>
        </div>
      </section>

      <section class="card" v-if="selectedBoard">
        <h2>Колонки</h2>
        <div class="stack">
          <input v-model="createColumnForm.name" placeholder="Название колонки" :disabled="!canManageBoard" />
          <input
            v-model="createColumnForm.kind"
            placeholder="Ключ: review, qa, done"
            :disabled="!canManageBoard"
          />
          <p class="hint-text">Ключ: `lowercase_snake_case` (например, `in_progress`).</p>
          <input
            v-model.number="createColumnForm.position"
            type="number"
            min="0"
            placeholder="Позиция"
            :disabled="!canManageBoard"
          />
          <p class="hint-text">Позиция: порядок колонки (0 — первая).</p>
          <button class="primary-btn" @click="createColumn" :disabled="!canManageBoard">Добавить колонку</button>

          <div class="column-admin-list" v-if="state.columns.length">
            <article class="column-admin-row" v-for="column in state.columns" :key="column.id">
              <div v-if="column.kind === 'backlog'" class="column-admin-system">
                <strong>{{ column.name }}</strong>
                <p class="field-help">Backlog — системная колонка: переименование, удаление и перенос задач при удалении отключены.</p>
              </div>
              <div v-else class="column-admin-grid">
                <input
                  v-model="columnDraftById[column.id].name"
                  class="column-admin-name"
                  placeholder="Название"
                  :disabled="!canManageBoard"
                />
                <input
                  v-model="columnDraftById[column.id].kind"
                  class="column-admin-kind"
                  placeholder="Ключ: review"
                  :disabled="!canManageBoard"
                />
                <input
                  v-model.number="columnDraftById[column.id].position"
                  class="column-admin-position"
                  type="number"
                  min="0"
                  placeholder="Позиция"
                  :disabled="!canManageBoard"
                />
              </div>
              <div v-if="column.kind !== 'backlog'" class="column-admin-actions">
                <select
                  v-model="columnDeleteTargetById[column.id]"
                  :disabled="!canManageBoard || state.columns.length <= 1"
                >
                  <option
                    v-for="targetColumn in state.columns.filter((item) => item.id !== column.id)"
                    :key="targetColumn.id"
                    :value="targetColumn.id"
                  >
                    {{ targetColumn.name }}
                  </option>
                </select>
                <p class="field-help">Колонка для переноса задач при удалении текущей.</p>
                <div class="column-admin-buttons">
                  <button class="ghost-btn" @click="updateBoardColumn(column.id)" :disabled="!canManageBoard">
                    Сохранить
                  </button>
                  <button
                    class="danger-btn"
                    @click="deleteBoardColumn(column.id)"
                    :disabled="!canManageBoard || !canDeleteBoardColumn(column.id)"
                  >
                    Удалить
                  </button>
                </div>
              </div>
              <p class="hint-text">Задач в колонке: {{ allTaskCountByColumn[column.id] || 0 }}</p>
            </article>
          </div>

          <p class="hint-text" v-if="!canManageBoard">Только владелец или наставник может создавать, редактировать и удалять колонки.</p>
        </div>
      </section>

      <section class="card" v-if="selectedBoard">
        <h2>Создать задачу</h2>
        <div class="stack">
          <input v-model="createTaskForm.title" placeholder="Название задачи" />
          <textarea v-model="createTaskForm.description" rows="3" placeholder="Описание задачи" />
          <select v-if="!isScrumBoard" v-model="createTaskForm.columnId">
            <option v-for="column in state.columns" :key="column.id" :value="column.id">
              {{ column.name }} ({{ formatColumnKindLabel(column.kind) }})
            </option>
          </select>
          <select v-else v-model="createTaskForm.sprintId">
            <option value="">Без спринта (в бэклог)</option>
            <option v-for="sprint in state.sprints" :key="sprint.id" :value="sprint.id">
              {{ sprint.name }} · {{ formatSprintStatusLabel(sprint.status) }}
            </option>
          </select>
          <select v-model="createTaskForm.assignee">
            <option value="">Не назначен</option>
            <option v-for="member in assigneeOptions" :key="member.email" :value="member.email">
              {{ member.label }}
            </option>
          </select>
          <input v-model="createTaskForm.dueDate" type="date" />
          <p class="field-help">Исполнитель сохраняется по email, выберите участника из списка.</p>
          <p v-if="isScrumBoard" class="field-help">Если выбран спринт, задача создается сразу в его колонке «К выполнению».</p>
          <button class="primary-btn" @click="createTask">Создать задачу</button>
        </div>
      </section>

      <section class="card" v-if="isScrumBoard">
        <h2>Спринты</h2>
        <div class="stack">
          <p class="field-help">Выбор активного спринта — в верхней части доски.</p>
          <div v-if="selectedSprint" class="sprint-current">
            <strong>{{ selectedSprint.name }}</strong>
            <span class="sprint-current-status">{{ formatSprintStatusLabel(selectedSprint.status) }}</span>
          </div>
          <p v-else class="hint-text">Выберите спринт, чтобы увидеть его статус и управлять им.</p>
          <input v-model="createSprintForm.name" placeholder="Название спринта" :disabled="!canManageSprint" />
          <input v-model="createSprintForm.goal" placeholder="Цель спринта" :disabled="!canManageSprint" />
          <div class="two-col">
            <input v-model="createSprintForm.startsAt" type="date" :disabled="!canManageSprint" />
            <input v-model="createSprintForm.endsAt" type="date" :disabled="!canManageSprint" />
          </div>
          <button class="primary-btn" @click="createSprint" :disabled="!canManageSprint">Создать спринт</button>
          <div class="two-col">
            <button
              class="ghost-btn"
              @click="startSprint"
              :disabled="!canManageSprint || !selectedSprint || selectedSprint.status !== 'planned'"
            >
              Запустить
            </button>
            <button
              class="ghost-btn"
              @click="closeSprint"
              :disabled="!canManageSprint || !selectedSprint || selectedSprint.status !== 'active'"
            >
              Закрыть
            </button>
          </div>
          <p class="hint-text" v-if="!canManageSprint">Только владелец или наставник может управлять спринтами.</p>
        </div>
      </section>
    </template>
  </aside>
</template>
