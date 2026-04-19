<template>
  <div class="app-shell">
    <div class="login-overlay" v-if="!token">
      <div class="login-card">
        <h1>{{ appTitle }}</h1>
        <p>输入您的用户 ID 以开始聊天</p>
        <input
          ref="loginInput"
          v-model="loginUserId"
          type="text"
          autocomplete="off"
          placeholder="用户 ID"
          v-on:keyup.enter="login"
        >
        <button class="btn btn-primary" v-on:click="login" v-bind:disabled="loggingIn">
          {{ loggingIn ? '登录中...' : '继续' }}
        </button>
        <div class="error" v-if="loginError">{{ loginError }}</div>
      </div>
    </div>

    <div class="layout" v-else>
      <aside class="sidebar" v-bind:class="{ open: mobileSidebarOpen, closed: !sidebarOpen }">
        <div class="sidebar-head">
          <button class="btn-icon" v-on:click="toggleSidebarDesktop" title="收起/展开菜单">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="3" y1="12" x2="21" y2="12"></line><line x1="3" y1="6" x2="21" y2="6"></line><line x1="3" y1="18" x2="21" y2="18"></line></svg>
          </button>
        </div>

        <div class="sidebar-nav">
          <button class="new-chat-gemini" v-on:click="createConversation()" v-bind:disabled="sending">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line></svg>
            <span v-if="sidebarOpen">新对话</span>
          </button>
          
          <div class="search-box-gemini" v-if="sidebarOpen">
            <div class="search-input-wrap">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>
              <input type="text" v-model="conversationQuery" placeholder="搜索对话">
            </div>
          </div>
        </div>

        <div class="conversation-list" v-on:click="activeDropdownId = null" v-if="sidebarOpen">
          <div
            v-for="item in filteredConversations"
            :key="item.id"
            class="conversation-item"
            v-bind:class="{ active: currentConversation && currentConversation.id === item.id }"
          >
            <div v-if="editingConversationId === item.id" class="sidebar-rename">
              <input v-model="conversationTitleDraft" v-on:keyup.enter="saveConversationTitle(item)" v-on:blur="saveConversationTitle(item)" v-on:keyup.esc="cancelEditConversationTitle" ref="titleInput">
            </div>
            <button v-else class="conversation-main" v-on:click="selectConversation(item.id)">
              <div class="conversation-title">{{ item.title }}</div>
            </button>
            <div class="conversation-actions" v-if="editingConversationId !== item.id">
              <div class="dropdown-wrap">
                <button class="conversation-more" title="更多操作" v-on:click.stop="toggleDropdown(item.id)">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="1"></circle><circle cx="12" cy="5" r="1"></circle><circle cx="12" cy="19" r="1"></circle></svg>
                </button>
                <div class="dropdown-menu" v-if="activeDropdownId === item.id">
                  <button v-on:click.stop="startEditConversationTitle(item)">
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4L18.5 2.5z"></path></svg>
                    重命名
                  </button>
                  <button class="danger" v-on:click.stop="deleteConversation(item.id)">
                    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
                    删除
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

      </aside>

      <div class="mobile-mask" v-if="mobileSidebarOpen" v-on:click="toggleSidebar"></div>

      <main class="main" v-on:click="activeDropdownId = null" v-bind:class="{ 'new-chat-layout': messages.length === 0 }">
        <header class="topbar">
          <div class="top-left">
            <div class="model-selector">
              <div class="model-tag topbar-title-hidden" aria-hidden="true"></div>
            </div>
          </div>
          <div class="top-right">
          </div>
        </header>

        <section class="messages" ref="messagesPanel">
          <div class="welcome" v-if="messages.length === 0 && !sending">
            <div class="welcome-title">今天我能帮您做些什么？</div>
            <div class="welcome-sub">我可以协助您完成写作、学习或构思创意</div>
          </div>

          <article
            class="message"
            v-for="message in messages"
            :key="message.id"
            v-bind:class="['role-' + message.role, { editing: editingMessageId === message.id }]"
          >
            <div class="message-avatar" v-if="message.role === 'assistant'">
              <img :src="modelAvatar(message.model_id || selectedModelId)" alt="AI">
            </div>

            <div class="message-main">
              <div class="message-header" v-if="message.role === 'assistant'">
                <div class="message-model-name">{{ modelName(message.model_id || selectedModelId) }}</div>
              </div>

              <div class="think-box" v-if="message.role === 'assistant' && message.think_content">
                <button class="think-toggle" v-on:click="toggleThink(message.id)">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" v-bind:style="{ transform: isThinkExpanded(message.id) ? 'rotate(90deg)' : 'none' }"><polyline points="9 18 15 12 9 6"></polyline></svg>
                  <span>深度思考</span>
                </button>
                <div class="think-content" v-if="isThinkExpanded(message.id)">{{ message.think_content }}</div>
                <div class="think-content" v-else>{{ thinkPreview(message.think_content) }}</div>
              </div>

              <div class="message-edit" v-if="editingMessageId === message.id">
                <div class="message-edit-card">
                  <textarea v-model="editingMessageDraft" ref="messageEditor"></textarea>
                  <div class="message-edit-actions">
                    <button class="btn btn-ghost btn-mini" v-on:click="cancelMessageEdit">取消</button>
                    <button class="btn btn-primary btn-mini" v-on:click="submitMessageEdit" v-bind:disabled="sending || !editingMessageDraft.trim()">更新</button>
                  </div>
                </div>
              </div>

              <div class="thinking-text" v-else-if="message.role === 'assistant' && sending && streamPendingId === message.id && !message.content_raw">Thinking...</div>
              <div class="message-body markdown-body" v-else v-html="message.content_html"></div>

              <div class="message-actions" v-if="editingMessageId !== message.id && !(sending && String(message.id).indexOf('pending') === 0)">
                <button class="btn-action" v-if="message.role === 'user'" v-on:click="startMessageEdit(message)" title="编辑">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4L18.5 2.5z"></path></svg>
                </button>
                <button class="btn-action" v-if="message.role === 'assistant'" v-on:click="regenerateFromAssistant(message)" title="重新生成">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"></polyline><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"></path></svg>
                </button>
                <button class="btn-action" v-on:click="copyMessageText(message)" title="复制">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
                </button>
                <button class="btn-action" v-on:click="deleteFromMessage(message)" title="删除此处及后续">
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"></polyline><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path></svg>
                </button>
              </div>
            </div>
          </article>
          <div class="messages-spacer"></div>
        </section>

        <footer class="composer-wrap">
          <div class="composer-card">
            <textarea
              ref="composer"
              v-model="draft"
              placeholder="给 AI 发送消息"
              v-on:keydown="onComposerKeydown"
              rows="1"
            ></textarea>

            <div class="composer-bottom">
              <div class="composer-left">
                <div class="model-switch-pill">
                  <select v-model="selectedModelId" v-bind:disabled="sending">
                    <option v-for="model in models" :key="model.id" :value="model.id">{{ model.name }}</option>
                  </select>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="6 9 12 15 18 9"></polyline></svg>
                </div>
              </div>
              <div class="composer-right">
                <button class="btn-send" v-on:click="sendMessage" v-bind:disabled="!canSend" v-if="!sending">
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><line x1="12" y1="19" x2="12" y2="5"></line><polyline points="5 12 12 5 19 12"></polyline></svg>
                </button>
                <button class="btn-send stop" v-on:click="stopStreaming" v-else>
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="7" y="7" width="10" height="10" fill="currentColor" stroke="none"></rect>
                  </svg>
                </button>
              </div>
            </div>
          </div>
          <div class="composer-error error" v-if="chatError">{{ chatError }}</div>
        </footer>
      </main>
    </div>

    <div class="confirm-overlay" v-if="confirmDialog.visible" v-on:click="closeConfirm">
      <div class="confirm-modal" v-on:click.stop>
        <div class="confirm-title">{{ confirmDialog.title }}</div>
        <div class="confirm-text">{{ confirmDialog.text }}</div>
        <div class="confirm-actions">
          <button class="btn btn-ghost" v-on:click="closeConfirm">取消</button>
          <button class="btn btn-primary confirm-danger" v-on:click="confirmProceed" v-bind:disabled="sending">{{ confirmDialog.confirmText }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
function createXHR(method, url, payload, token, callback) {
  var xhr = new XMLHttpRequest()
  xhr.open(method, url, true)
  xhr.setRequestHeader('Accept', 'application/json')
  if (payload) {
    xhr.setRequestHeader('Content-Type', 'application/json')
  }
  if (token) {
    xhr.setRequestHeader('Authorization', 'Bearer ' + token)
  }
  xhr.onreadystatechange = function () {
    var body
    if (xhr.readyState !== 4) {
      return
    }
    try {
      body = xhr.responseText ? JSON.parse(xhr.responseText) : null
    } catch (e) {
      body = null
    }
    if (xhr.status >= 200 && xhr.status < 300) {
      callback(null, body)
      return
    }
    callback(new Error(body && body.error ? body.error : ('HTTP ' + xhr.status)), body)
  }
  xhr.send(payload ? JSON.stringify(payload) : null)
}

function createXHRStream(url, payload, token, handlers) {
  var xhr = new XMLHttpRequest()
  var processed = 0
  var doneCalled = false
  var buffer = ''

  function parseEvents(chunk, flushAll) {
    var blocks
    var i
    buffer += chunk.replace(/\r\n/g, '\n').replace(/\r/g, '\n')
    blocks = buffer.split(/\n\n+/)
    if (!flushAll) {
      buffer = blocks.pop() || ''
    } else {
      buffer = ''
    }
    for (i = 0; i < blocks.length; i += 1) {
      var block = blocks[i]
      var lines
      var eventName = 'message'
      var dataLines = []
      var j
      var data
      var rawData
      if (!block || !block.replace(/\s+/g, '')) {
        continue
      }
      lines = block.split('\n')
      for (j = 0; j < lines.length; j += 1) {
        if (lines[j].indexOf('event:') === 0) {
          eventName = lines[j].slice(6).replace(/^\s+|\s+$/g, '')
        } else if (lines[j].indexOf('data:') === 0) {
          dataLines.push(lines[j].slice(5).replace(/^\s+/, ''))
        }
      }
      rawData = dataLines.join('\n')
      if (!rawData) {
        continue
      }
      try {
        data = JSON.parse(rawData)
      } catch (e) {
        data = { error: 'invalid stream payload' }
      }
      if (handlers && handlers.onEvent) {
        handlers.onEvent(eventName, data)
      }
    }
  }

  xhr.open('POST', url, true)
  xhr.setRequestHeader('Accept', 'text/event-stream')
  xhr.setRequestHeader('Content-Type', 'application/json')
  if (token) {
    xhr.setRequestHeader('Authorization', 'Bearer ' + token)
  }
  xhr.onprogress = function () {
    var text
    if (!xhr.responseText || xhr.responseText.length <= processed) {
      return
    }
    text = xhr.responseText.slice(processed)
    processed = xhr.responseText.length
    parseEvents(text)
  }
  xhr.onreadystatechange = function () {
    if (xhr.readyState !== 4) {
      return
    }
    if (xhr.responseText && xhr.responseText.length > processed) {
      parseEvents(xhr.responseText.slice(processed), true)
      processed = xhr.responseText.length
    }
    if (xhr.status >= 200 && xhr.status < 300) {
      if (!doneCalled && handlers && handlers.onDone) {
        doneCalled = true
        handlers.onDone()
      }
      return
    }
    if (handlers && handlers.onError) {
      handlers.onError(new Error('HTTP ' + xhr.status))
    }
  }
  xhr.send(JSON.stringify(payload || {}))
  return xhr
}

function escapeHTML(text) {
  return String(text)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function previewLines(text, count) {
  var lines
  if (!text) return ''
  lines = String(text).replace(/\r\n/g, '\n').split('\n')
  if (lines.length <= count) return lines.join('\n')
  return lines.slice(lines.length - count).join('\n')
}

function suffixPrefixLength(value, token) {
  var max = Math.min(value.length, token.length - 1)
  var i
  var lowerValue = value.toLowerCase()
  var lowerToken = token.toLowerCase()
  for (i = max; i > 0; i -= 1) {
    if (lowerToken.indexOf(lowerValue.slice(lowerValue.length - i)) === 0) {
      return i
    }
  }
  return 0
}

function splitThinkFromContentDelta(delta, parserState) {
  var openTag = '<think>'
  var closeTag = '</think>'
  var state = parserState || { inThink: false, pending: '' }
  var source = String((state.pending || '') + (delta || ''))
  var lower = source.toLowerCase()
  var index = 0
  var answer = ''
  var think = ''
  var inThink = !!state.inThink
  var pending = ''
  var marker
  var hold
  while (index < source.length) {
    if (!inThink) {
      marker = lower.indexOf(openTag, index)
      if (marker < 0) {
        hold = suffixPrefixLength(source.slice(index), openTag)
        answer += source.slice(index, source.length - hold)
        pending = source.slice(source.length - hold)
        break
      }
      answer += source.slice(index, marker)
      inThink = true
      index = marker + openTag.length
      continue
    }
    marker = lower.indexOf(closeTag, index)
    if (marker < 0) {
      hold = suffixPrefixLength(source.slice(index), closeTag)
      think += source.slice(index, source.length - hold)
      pending = source.slice(source.length - hold)
      break
    }
    think += source.slice(index, marker)
    inThink = false
    index = marker + closeTag.length
  }
  return {
    contentDelta: answer,
    thinkDelta: think,
    inThink: inThink,
    pending: pending
  }
}

export default {
  name: 'App',
  data: function () {
    return {
      appTitle: 'Web AI',
      token: '',
      currentUser: '',
      loginUserId: '',
      loginError: '',
      loggingIn: false,
      models: [],
      selectedModelId: '',
      conversations: [],
      conversationQuery: '',
      currentConversation: null,
      messages: [],
      draft: '',
      sending: false,
      chatError: '',
      mobileSidebarOpen: false,
      sidebarOpen: true,
      titleRefreshTimer: 0,
      activeStreamXHR: null,
      streamPendingId: '',
      streamPendingUserId: '',
      streamStoppedByUser: false,
      streamThinkExpanded: {},
      editingMessageId: 0,
      editingMessageDraft: '',
      editingConversationId: '',
      activeDropdownId: null,
      conversationTitleDraft: '',
      confirmDialog: {
        visible: false,
        title: '',
        text: '',
        confirmText: '确认',
        action: '',
        payload: null
      }
    }
  },
  computed: {
    trimmedDraft: function () {
      return this.draft.replace(/^\s+|\s+$/g, '')
    },
    canSend: function () {
      return !this.sending && !!this.trimmedDraft
    },
    filteredConversations: function () {
      var query = this.conversationQuery.replace(/^\s+|\s+$/g, '').toLowerCase()
      if (!query) {
        return this.conversations
      }
      return this.conversations.filter(function (item) {
        return String(item.title || '').toLowerCase().indexOf(query) >= 0
      })
    },
    currentUserInitial: function () {
      return this.currentUser ? this.currentUser.slice(0, 1).toUpperCase() : 'U'
    },
    selectedModelName: function () {
      return this.modelName(this.selectedModelId)
    }
  },
  watch: {
    draft: function () {
      this.resizeComposer()
    }
  },
  mounted: function () {
    this.fetchPublicConfig()
    this.focusLogin()
  },
  beforeDestroy: function () {
    this.clearTitleRefreshTimer()
    this.abortActiveStream()
  },
  methods: {
    resizeComposer: function () {
      var self = this
      this.$nextTick(function () {
        var el = self.$refs.composer
        if (!el) return
        el.style.height = 'auto'
        el.style.height = Math.min(el.scrollHeight, 200) + 'px'
      })
    },
    fetchPublicConfig: function () {
      var self = this
      createXHR('GET', '/api/public/config', null, '', function (err, data) {
        if (!err && data && data.title) {
          self.appTitle = data.title
          document.title = data.title
        }
      })
    },
    focusLogin: function () {
      var self = this
      this.$nextTick(function () {
        if (self.$refs.loginInput) {
          self.$refs.loginInput.focus()
        }
      })
    },
    focusComposer: function () {
      var self = this
      this.$nextTick(function () {
        if (self.$refs.composer) {
          self.$refs.composer.focus()
        }
      })
    },
    focusMessageEditor: function () {
      var self = this
      this.$nextTick(function () {
        var editor = self.$refs.messageEditor
        if (editor && editor.length) {
          editor = editor[0]
        }
        if (editor && editor.focus) {
          editor.focus()
        }
      })
    },
    login: function () {
      var self = this
      if (this.loggingIn) {
        return
      }
      this.loginError = ''
      if (!this.loginUserId.replace(/^\s+|\s+$/g, '')) {
        this.loginError = 'User ID is required.'
        return
      }
      this.loggingIn = true
      createXHR('POST', '/api/session/login', { user_id: this.loginUserId }, '', function (err, data) {
        self.loggingIn = false
        if (err) {
          self.loginError = err.message
          self.focusLogin()
          return
        }
        self.token = data.token
        self.currentUser = data.user_id
        self.loadModels(function () {
          self.loadConversations(function () {
            if (self.conversations.length > 0) {
              self.selectConversation(self.conversations[0].id)
            } else {
              self.createConversation()
            }
          })
        })
      })
    },
    loadModels: function (done) {
      var self = this
      createXHR('GET', '/api/models', null, this.token, function (err, data) {
        if (err) {
          self.chatError = err.message
          if (done) done()
          return
        }
        self.models = data.models || []
        self.selectedModelId = data.default_model || (self.models[0] && self.models[0].id) || ''
        if (done) done()
      })
    },
    loadConversations: function (done) {
      var self = this
      createXHR('GET', '/api/conversations', null, this.token, function (err, data) {
        if (err) {
          self.chatError = err.message
          if (done) done()
          return
        }
        self.conversations = data.conversations || []
        if (self.currentConversation) {
          self.syncCurrentConversation()
        }
        if (done) done()
      })
    },
    syncCurrentConversation: function () {
      var i
      for (i = 0; i < this.conversations.length; i += 1) {
        if (this.currentConversation && this.currentConversation.id === this.conversations[i].id) {
          this.currentConversation = this.conversations[i]
          return
        }
      }
    },
    createConversation: function (done) {
      var self = this
      this.abortActiveStream()
      this.streamPendingId = ''
      this.streamPendingUserId = ''
      createXHR('POST', '/api/conversations', { model_id: this.selectedModelId }, this.token, function (err, data) {
        if (err) {
          self.chatError = err.message
          if (done) done(err)
          return
        }
        self.conversations.unshift(data)
        self.currentConversation = data
        self.selectedModelId = data.model_id
        self.messages = []
        self.chatError = ''
        self.editingMessageId = 0
        self.mobileSidebarOpen = false
        self.focusComposer()
        if (done) done(null, data)
      })
    },
    selectConversation: function (conversationId) {
      var self = this
      this.abortActiveStream()
      this.streamPendingId = ''
      this.streamPendingUserId = ''
      this.sending = false
      this.editingMessageId = 0
      createXHR('GET', '/api/conversations/' + conversationId + '/messages', null, this.token, function (err, data) {
        if (err) {
          self.chatError = err.message
          return
        }
        self.currentConversation = data.conversation
        self.selectedModelId = data.conversation.model_id
        self.messages = data.messages || []
        self.mobileSidebarOpen = false
        self.chatError = ''
        self.scrollMessages(true)
        self.focusComposer()
      })
    },
    deleteConversation: function (conversationId) {
      if (!conversationId) {
        return
      }
      this.openConfirm({
        title: '删除对话',
        text: '删除后将无法恢复，确认删除这个会话吗？',
        confirmText: '删除',
        action: 'delete_conversation',
        payload: conversationId
      })
    },
    performDeleteConversation: function (conversationId) {
      var self = this
      var nextId = ''
      var i
      if (this.currentConversation && this.currentConversation.id === conversationId) {
        this.abortActiveStream()
        this.streamPendingId = ''
        this.streamPendingUserId = ''
        this.sending = false
      }
      createXHR('DELETE', '/api/conversations/' + conversationId, null, this.token, function (err) {
        if (err) {
          self.chatError = err.message
          return
        }
        for (i = 0; i < self.conversations.length; i += 1) {
          if (self.conversations[i].id === conversationId) {
            self.conversations.splice(i, 1)
            break
          }
        }
        if (self.currentConversation && self.currentConversation.id === conversationId) {
          self.currentConversation = null
          self.messages = []
          if (self.conversations.length > 0) {
            nextId = self.conversations[0].id
          }
        }
        if (nextId) {
          self.selectConversation(nextId)
        } else if (!self.currentConversation) {
          self.createConversation()
        }
      })
    },
    sendMessage: function () {
      this.sendMessageCore(this.trimmedDraft, this.selectedModelId)
    },
    sendMessageCore: function (text, forcedModelId) {
      var self = this
      var conversationId
      var optimisticUser
      var optimisticUserId
      var pendingAssistant
      var streamCompleted = false
      var stickBottom = this.isNearBottom()
      var targetModelId
      text = String(text || '').replace(/^\s+|\s+$/g, '')
      if (this.sending || !text) {
        return
      }
      this.chatError = ''
      targetModelId = forcedModelId || this.selectedModelId
      if (!this.currentConversation) {
        this.createConversation(function (err) {
          if (!err) {
            self.sendMessageCore(text, targetModelId)
          }
        })
        return
      }

      this.clearTitleRefreshTimer()
      this.draft = ''
      this.editingMessageId = 0
      conversationId = this.currentConversation.id
      optimisticUser = {
        id: 'pending-user-' + new Date().getTime(),
        role: 'user',
        model_id: targetModelId,
        content_raw: text,
        content_html: '<p>' + escapeHTML(text).replace(/\n/g, '<br>') + '</p>',
        created_at: new Date().toISOString()
      }
      optimisticUserId = optimisticUser.id
      pendingAssistant = {
        id: 'pending-assistant-' + new Date().getTime(),
        role: 'assistant',
        model_id: targetModelId,
        content_raw: '',
        content_html: '<p></p>',
        think_content: '',
        _inThink: false,
        _tagBuffer: '',
        created_at: new Date().toISOString()
      }
      this.messages.push(optimisticUser)
      this.messages.push(pendingAssistant)
      this.sending = true
      this.streamStoppedByUser = false
      this.streamPendingUserId = optimisticUserId
      this.streamPendingId = pendingAssistant.id
      this.abortActiveStream()
      this.scrollMessages(true)

      this.activeStreamXHR = createXHRStream('/api/chat/completions', {
        conversation_id: conversationId,
        model_id: targetModelId,
        message: text,
        stream: true
      }, this.token, {
        onEvent: function (eventName, data) {
          var i
          var pending
          var parsed
          if (!self.streamPendingId) return
          if (eventName === 'error') {
            self.chatError = (data && data.error) ? data.error : 'stream error'
            return
          }
          if (eventName === 'delta') {
            for (i = 0; i < self.messages.length; i += 1) {
              if (self.messages[i].id === self.streamPendingId) {
                pending = self.messages[i]
                break
              }
            }
            if (!pending) return
            if (data && data.content_delta) {
              parsed = splitThinkFromContentDelta(data.content_delta, {
                inThink: !!pending._inThink,
                pending: pending._tagBuffer || ''
              })
              pending._inThink = parsed.inThink
              pending._tagBuffer = parsed.pending
              if (parsed.contentDelta) {
                pending.content_raw = (pending.content_raw || '') + parsed.contentDelta
              }
              if (parsed.thinkDelta) {
                pending.think_content = (pending.think_content || '') + parsed.thinkDelta
              }
            }
            if (data && data.think_delta) {
              pending.think_content = (pending.think_content || '') + data.think_delta
            }
            pending.content_html = '<p>' + escapeHTML(pending.content_raw || '').replace(/\n/g, '<br>') + '</p>'
            if (stickBottom || self.isNearBottom()) {
              self.scrollMessages(true)
            }
          }
          if (eventName === 'done') {
            streamCompleted = true
            self.finishStreamMessage(data, conversationId)
          }
        },
        onDone: function () {
          self.activeStreamXHR = null
          if (self.streamStoppedByUser) {
            self.streamStoppedByUser = false
            return
          }
          if (!streamCompleted) {
            self.sending = false
            self.removePendingAssistant()
            self.removeMessageById(optimisticUserId)
            if (self.streamPendingUserId === optimisticUserId) {
              self.streamPendingUserId = ''
            }
            self.reloadConversationAndRecover(conversationId)
            self.chatError = self.chatError || '流式结束但未收到 done，已自动回补'
          }
        },
        onError: function (err) {
          self.activeStreamXHR = null
          if (self.streamStoppedByUser) {
            self.streamStoppedByUser = false
            return
          }
          self.sending = false
          self.removePendingAssistant()
          self.removeMessageById(optimisticUserId)
          if (self.streamPendingUserId === optimisticUserId) {
            self.streamPendingUserId = ''
          }
          self.draft = text
          self.chatError = err && err.message ? err.message : 'stream error'
        }
      })
    },
    onComposerKeydown: function (event) {
      if (event.keyCode === 13 && !event.shiftKey) {
        event.preventDefault()
        this.sendMessage()
      }
    },
    stopStreaming: function () {
      this.streamStoppedByUser = true
      this.abortActiveStream()
      this.finalizeStoppedStream()
    },
    finishStreamMessage: function (data, conversationId) {
      var i
      var doneMessage = data && data.message ? data.message : null
      var self = this
      this.sending = false
      this.activeStreamXHR = null
      if (data && data.conversation) {
        this.currentConversation = data.conversation
      }
      for (i = this.messages.length - 1; i >= 0; i -= 1) {
        if (this.messages[i].id === this.streamPendingId) {
          if (doneMessage) {
            this.messages.splice(i, 1, doneMessage)
          } else {
            this.messages.splice(i, 1)
          }
          break
        }
      }
      if (this.streamPendingUserId) {
        this.removeMessageById(this.streamPendingUserId)
      }
      this.streamPendingId = ''
      this.streamPendingUserId = ''
      this.reloadCurrentConversation(function () {
        self.refreshTitleUntilReady(conversationId)
        self.focusComposer()
      })
    },
    startMessageEdit: function (message) {
      if (this.sending || message.role !== 'user') {
        return
      }
      this.editingMessageId = message.id
      this.editingMessageDraft = message.content_raw
      this.focusMessageEditor()
    },
    cancelMessageEdit: function () {
      this.editingMessageId = 0
      this.editingMessageDraft = ''
      this.focusComposer()
    },
    submitMessageEdit: function () {
      var self = this
      var text = this.editingMessageDraft.replace(/^\s+|\s+$/g, '')
      var messageId = this.editingMessageId
      var targetModelId = this.selectedModelId
      if (!messageId || !text || this.sending) {
        return
      }
      this.chatError = ''
      this.truncateFromMessage(messageId, function (err) {
        if (err) {
          self.chatError = err.message
          return
        }
        self.cancelMessageEdit()
        self.sendMessageCore(text, targetModelId)
      })
    },
    deleteFromMessage: function (message) {
      if (this.sending) {
        return
      }
      if (!message || !message.id) {
        return
      }
      this.openConfirm({
        title: '删除消息',
        text: '将删除此消息及其后续内容，是否继续？',
        confirmText: '删除',
        action: 'delete_message_from',
        payload: message.id
      })
    },
    performDeleteFromMessage: function (messageId) {
      var self = this
      this.truncateFromMessage(messageId, function (err) {
        if (err) {
          self.chatError = err.message
        }
      })
    },
    openConfirm: function (options) {
      this.activeDropdownId = null
      this.confirmDialog.visible = true
      this.confirmDialog.title = options && options.title ? options.title : '请确认'
      this.confirmDialog.text = options && options.text ? options.text : '确认继续该操作吗？'
      this.confirmDialog.confirmText = options && options.confirmText ? options.confirmText : '确认'
      this.confirmDialog.action = options && options.action ? options.action : ''
      this.confirmDialog.payload = options ? options.payload : null
    },
    closeConfirm: function () {
      this.confirmDialog.visible = false
      this.confirmDialog.title = ''
      this.confirmDialog.text = ''
      this.confirmDialog.confirmText = '确认'
      this.confirmDialog.action = ''
      this.confirmDialog.payload = null
    },
    confirmProceed: function () {
      var action = this.confirmDialog.action
      var payload = this.confirmDialog.payload
      this.closeConfirm()
      if (action === 'delete_conversation') {
        this.performDeleteConversation(payload)
        return
      }
      if (action === 'delete_message_from') {
        this.performDeleteFromMessage(payload)
      }
    },
    regenerateFromAssistant: function (assistantMessage) {
      var self = this
      var promptMessage
      var targetModelId = this.selectedModelId
      if (this.sending || assistantMessage.role !== 'assistant') {
        return
      }
      promptMessage = this.findPreviousUserMessage(assistantMessage.id)
      if (!promptMessage) {
        this.chatError = '未找到可重生成的用户消息。'
        return
      }
      this.truncateFromMessage(promptMessage.id, function (err) {
        if (err) {
          self.chatError = err.message
          return
        }
        self.sendMessageCore(promptMessage.content_raw, targetModelId)
      })
    },
    findPreviousUserMessage: function (messageId) {
      var i
      for (i = this.messages.length - 1; i >= 0; i -= 1) {
        if (this.messages[i].id === messageId) {
          break
        }
      }
      for (i = i - 1; i >= 0; i -= 1) {
        if (this.messages[i].role === 'user') {
          return this.messages[i]
        }
      }
      return null
    },
    truncateFromMessage: function (messageId, done) {
      var self = this
      if (messageId.toString().indexOf('pending') === 0) {
        var idx = -1
        for (var i = 0; i < self.messages.length; i++) {
          if (self.messages[i].id === messageId) {
            idx = i
            break
          }
        }
        if (idx !== -1) {
          self.messages.splice(idx)
        }
        if (done) done(null)
        return
      }
      createXHR('DELETE', '/api/messages/' + messageId + '?truncate=true', null, this.token, function (err) {
        if (!err) {
          var idx = -1
          for (var i = 0; i < self.messages.length; i++) {
            if (self.messages[i].id === messageId) {
              idx = i
              break
            }
          }
          if (idx !== -1) {
            self.messages.splice(idx)
          }
        }
        if (done) done(err || null)
      })
    },
    startEditConversationTitle: function (item) {
      var self = this
      this.activeDropdownId = null
      this.editingConversationId = item.id
      this.conversationTitleDraft = item.title
      this.$nextTick(function () {
        var inputs = self.$refs.titleInput
        if (inputs && inputs.length) {
          inputs[0].focus()
        } else if (inputs) {
          inputs.focus()
        }
      })
    },
    cancelEditConversationTitle: function () {
      this.editingConversationId = ''
      this.conversationTitleDraft = ''
    },
    saveConversationTitle: function (item) {
      var self = this
      var title
      if (this.editingConversationId !== item.id) return
      title = this.conversationTitleDraft.replace(/^\s+|\s+$/g, '')
      if (!title || title === item.title) {
        this.editingConversationId = ''
        return
      }
      createXHR('PATCH', '/api/conversations/' + item.id, { title: title }, this.token, function (err, data) {
        if (err) {
          self.chatError = err.message
          return
        }
        if (self.currentConversation && self.currentConversation.id === item.id) {
          self.currentConversation = data
        }
        self.editingConversationId = ''
        self.loadConversations(function () {})
      })
    },
    copyMessageText: function (message) {
      var text = String(message.content_raw || '').replace(/^\s+|\s+$/g, '')
      if (!text) return
      if (typeof navigator !== 'undefined' && navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text)
        return
      }
      try {
        var area = document.createElement('textarea')
        area.value = text
        document.body.appendChild(area)
        area.select()
        document.execCommand('copy')
        document.body.removeChild(area)
      } catch (e) {}
    },
    reloadCurrentConversation: function (done) {
      var self = this
      if (!this.currentConversation) {
        if (done) done(new Error('conversation missing'))
        return
      }
      createXHR('GET', '/api/conversations/' + this.currentConversation.id + '/messages', null, this.token, function (err, data) {
        if (err) {
          if (done) done(err)
          return
        }
        self.currentConversation = data.conversation
        self.messages = data.messages || []
        self.loadConversations(function () {})
        self.scrollMessages(true)
        if (done) done(null)
      })
    },
    reloadConversationAndRecover: function (conversationId) {
      var self = this
      this.loadConversations(function () {
        self.selectConversation(conversationId)
        self.refreshTitleUntilReady(conversationId)
      })
    },
    refreshTitleUntilReady: function (conversationId) {
      var self = this
      var attempts = 0
      var maxAttempts = 60
      var intervalMs = 1500
      function shouldStop() {
        if (!self.currentConversation) return true
        if (self.currentConversation.id !== conversationId) return true
        return self.currentConversation.title && self.currentConversation.title !== '新对话'
      }
      function tick() {
        if (shouldStop()) {
          self.clearTitleRefreshTimer()
          return
        }
        attempts += 1
        self.loadConversations(function () {
          if (shouldStop() || attempts >= maxAttempts) {
            self.clearTitleRefreshTimer()
            return
          }
          self.titleRefreshTimer = window.setTimeout(tick, intervalMs)
        })
      }
      this.clearTitleRefreshTimer()
      this.titleRefreshTimer = window.setTimeout(tick, intervalMs)
    },
    clearTitleRefreshTimer: function () {
      if (this.titleRefreshTimer) {
        window.clearTimeout(this.titleRefreshTimer)
        this.titleRefreshTimer = 0
      }
    },
    abortActiveStream: function () {
      if (this.activeStreamXHR) {
        try {
          this.activeStreamXHR.abort()
        } catch (e) {}
        this.activeStreamXHR = null
      }
    },
    finalizeStoppedStream: function () {
      var i
      var pending
      var hasPartial
      var conversationId
      var self = this
      this.sending = false
      if (!this.streamPendingId) {
        this.streamPendingUserId = ''
        return
      }
      for (i = 0; i < this.messages.length; i += 1) {
        if (this.messages[i].id === this.streamPendingId) {
          pending = this.messages[i]
          break
        }
      }
      if (!pending) {
        this.streamPendingId = ''
        return
      }
      if (pending._tagBuffer) {
        if (pending._inThink) {
          pending.think_content = (pending.think_content || '') + pending._tagBuffer
        } else {
          pending.content_raw = (pending.content_raw || '') + pending._tagBuffer
        }
      }
      pending._tagBuffer = ''
      pending._inThink = false
      hasPartial = !!((pending.content_raw || '').replace(/^\s+|\s+$/g, '') || (pending.think_content || '').replace(/^\s+|\s+$/g, ''))
      conversationId = this.currentConversation ? this.currentConversation.id : ''
      if (!hasPartial) {
        this.removePendingAssistant()
        if (this.streamPendingUserId) {
          this.removeMessageById(this.streamPendingUserId)
          this.streamPendingUserId = ''
        }
        if (conversationId) {
          this.reloadConversationAndRecover(conversationId)
        }
        this.focusComposer()
        return
      }
      this.streamPendingId = ''
      this.streamPendingUserId = ''
      if (!conversationId) {
        this.focusComposer()
        return
      }
      this.persistPartialAssistant(conversationId, pending, function (err) {
        if (err) {
          self.chatError = err.message || '保存中断输出失败'
        }
        self.reloadConversationAndRecover(conversationId)
      })
      this.focusComposer()
    },
    persistPartialAssistant: function (conversationId, pendingMessage, done) {
      if (!conversationId || !pendingMessage) {
        if (done) done(new Error('conversation missing'))
        return
      }
      createXHR('POST', '/api/messages/partial', {
        conversation_id: conversationId,
        model_id: pendingMessage.model_id || this.selectedModelId,
        content: pendingMessage.content_raw || '',
        think_content: pendingMessage.think_content || ''
      }, this.token, function (err) {
        if (done) done(err || null)
      })
    },
    removePendingAssistant: function () {
      var i
      for (i = this.messages.length - 1; i >= 0; i -= 1) {
        if (this.messages[i].id === this.streamPendingId) {
          this.messages.splice(i, 1)
          break
        }
      }
      this.streamPendingId = ''
    },
    removeMessageById: function (messageId) {
      var i
      for (i = this.messages.length - 1; i >= 0; i -= 1) {
        if (this.messages[i].id === messageId) {
          this.messages.splice(i, 1)
          break
        }
      }
    },
    isNearBottom: function () {
      var panel = this.$refs.messagesPanel
      if (!panel) return true
      return (panel.scrollHeight - panel.scrollTop - panel.clientHeight) < 120
    },
    scrollMessages: function (force) {
      var self = this
      this.$nextTick(function () {
        var panel = self.$refs.messagesPanel
        if (!panel) return
        if (force || self.isNearBottom()) {
          panel.scrollTop = panel.scrollHeight
        }
      })
    },
    toggleSidebarDesktop: function () {
      this.sidebarOpen = !this.sidebarOpen
    },
    toggleDropdown: function (id) {
      if (this.activeDropdownId === id) {
        this.activeDropdownId = null
      } else {
        this.activeDropdownId = id
      }
    },
    closeDropdown: function () {
      this.activeDropdownId = null
    },
    toggleThink: function (messageId) {
      this.$set(this.streamThinkExpanded, String(messageId), !this.isThinkExpanded(messageId))
    },
    isThinkExpanded: function (messageId) {
      return !!this.streamThinkExpanded[String(messageId)]
    },
    thinkPreview: function (thinkText) {
      return previewLines(thinkText, 5)
    },
    toggleSidebar: function () {
      this.mobileSidebarOpen = !this.mobileSidebarOpen
    },
    modelName: function (modelId) {
	  var model = this.modelMeta(modelId)
	  if (model && model.name) {
		return model.name
	  }
	  return modelId || 'Assistant'
	},
	modelMeta: function (modelId) {
	  var i
	  for (i = 0; i < this.models.length; i += 1) {
		if (this.models[i].id === modelId) {
		  return this.models[i]
		}
	  }
	  return null
    },
    modelAvatar: function (modelId) {
	  var model = this.modelMeta(modelId)
	  if (model && model.avatar) return model.avatar
      var lower = String(modelId || '').toLowerCase()
      if (lower.indexOf('gpt') >= 0 || lower.indexOf('openai') >= 0) return '/avatars/brands/openai.svg'
      if (lower.indexOf('gemini') >= 0) return '/avatars/brands/gemini.svg'
      if (lower.indexOf('claude') >= 0 || lower.indexOf('anthropic') >= 0) return '/avatars/brands/claude.svg'
      if (lower.indexOf('deepseek') >= 0) return '/avatars/brands/deepseek.svg'
      if (lower.indexOf('qwen') >= 0 || lower.indexOf('tongyi') >= 0) return '/avatars/brands/qwen.svg'
      if (lower.indexOf('zhipu') >= 0 || lower.indexOf('glm') >= 0) return '/avatars/brands/zhipu.svg'
      if (lower.indexOf('grok') >= 0) return '/avatars/brands/grok.svg'
      if (lower.indexOf('xai') >= 0) return '/avatars/brands/xai.svg'
      return '/avatars/default.svg'
    },
    formatTime: function (value) {
      var date
      if (!value) return ''
      date = new Date(value)
      if (isNaN(date.getTime())) return ''
      return date.getHours() + ':' + ('0' + date.getMinutes()).slice(-2)
    }
  }
}
</script>

<style>
:root {
  --bg-sidebar: #131314;
  --bg-main: #0e0e10;
  --bg-message-user: #282829;
  --bg-input: #1e1f20;
  --text-main: #e3e3e3;
  --text-secondary: #c4c7c5;
  --border-color: #444746;
  --accent-color: #8ab4f8;
  --hover-bg: rgba(255, 255, 255, 0.08);
  --sidebar-width: 260px;
  --font-family: "Google Sans", "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
}

html,
body,
#app {
  margin: 0;
  padding: 0;
  height: 100%;
  width: 100%;
  overflow: hidden;
  background: var(--bg-main);
  color: var(--text-main);
  font-family: var(--font-family);
  -webkit-font-smoothing: antialiased;
}

* {
  box-sizing: border-box;
}

button,
input,
textarea,
select {
  font: inherit;
  color: inherit;
}

svg {
  display: block;
}

.app-shell {
  height: 100vh;
  width: 100vw;
  display: flex;
  overflow: hidden;
  background: var(--bg-main);
}

.layout {
  display: flex;
  width: 100%;
  height: 100%;
  overflow: hidden;
}

/* Sidebar */
.sidebar {
  width: var(--sidebar-width);
  height: 100%;
  background: var(--bg-sidebar);
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  transition: width 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  z-index: 10;
  overflow: hidden;
}

.sidebar.closed {
  width: 68px;
}

.sidebar-head {
  padding: 12px 14px;
  display: flex;
  align-items: center;
}

.sidebar-nav {
  padding: 8px 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.new-chat-gemini {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background: #1e1f20;
  border: 0;
  border-radius: 16px;
  color: #c4c7c5;
  cursor: pointer;
  transition: background 0.2s, color 0.2s;
  width: 100%;
  white-space: nowrap;
}

.sidebar.closed .new-chat-gemini {
  justify-content: center;
  padding: 12px 0;
  border-radius: 50%;
  width: 44px;
  height: 44px;
  margin: 0 auto;
}

.new-chat-gemini:hover {
  background: #333;
  color: #fff;
}

.new-chat-gemini span {
  font-size: 14px;
  font-weight: 500;
}

.search-box-gemini {
  padding: 4px 0;
}

.search-input-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
  background: #1e1f20;
  border-radius: 20px;
  padding: 10px 14px;
  color: var(--text-secondary);
}

.search-input-wrap input {
  background: transparent;
  border: 0;
  outline: none;
  font-size: 14px;
  width: 100%;
  color: #fff;
}

.conversation-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px 12px;
}

.conversation-item {
  margin-bottom: 2px;
  border-radius: 20px;
  display: flex;
  align-items: center;
  position: relative;
  transition: background 0.2s;
}

.conversation-item:hover {
  background: #1e1f20;
}

.conversation-item.active {
  background: #2d2e30;
}

.conversation-main {
  flex: 1;
  padding: 10px 16px;
  border: 0;
  text-align: left;
  background: transparent;
  cursor: pointer;
  min-width: 0;
}

.conversation-title {
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: #e3e3e3;
}

.conversation-actions {
  display: flex;
  align-items: center;
  padding-right: 8px;
  opacity: 0;
  transition: opacity 0.2s;
}

.conversation-item:hover .conversation-actions,
.dropdown-wrap:focus-within .conversation-actions,
.conversation-actions:hover {
  opacity: 1;
}

.conversation-more {
  background: transparent;
  border: 0;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 6px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.conversation-more:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
}

.dropdown-wrap {
  position: relative;
}

.dropdown-menu {
  position: absolute;
  top: 100%;
  right: 0;
  background: #1e1f20;
  border: 1px solid #444746;
  border-radius: 12px;
  padding: 8px;
  z-index: 20;
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 140px;
  box-shadow: 0 8px 24px rgba(0,0,0,0.5);
}

.dropdown-menu button {
  background: transparent;
  border: 0;
  padding: 10px 12px;
  text-align: left;
  border-radius: 8px;
  cursor: pointer;
  font-size: 14px;
  color: #e3e3e3;
  display: flex;
  align-items: center;
  gap: 12px;
}

.dropdown-menu button:hover {
  background: rgba(255, 255, 255, 0.08);
}

.dropdown-menu button.danger {
  color: #f28b82;
}

.sidebar-rename {
  flex: 1;
  padding: 4px 12px;
}

.sidebar-rename input {
  width: 100%;
  background: #1e1f20;
  border: 1px solid var(--accent-color);
  color: #fff;
  border-radius: 8px;
  padding: 6px 10px;
  font-size: 14px;
  outline: none;
}

.sidebar-footer {
  padding: 16px;
  border-top: 1px solid #444746;
}

.user-profile {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px;
  border-radius: 12px;
}

.user-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: #3c4043;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 500;
  color: #fff;
}

.user-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sidebar.closed .sidebar-footer {
  display: none;
}

/* Main Content */
.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  height: 100%;
  position: relative;
  overflow: hidden;
  background: var(--bg-main);
  transition: all 0.3s;
}

.topbar {
  height: 64px;
  padding: 0 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
  z-index: 5;
}

.top-left {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.btn-icon {
  background: transparent;
  border: 0;
  padding: 10px;
  border-radius: 50%;
  cursor: pointer;
  color: #c4c7c5;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.2s;
}

.btn-icon:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #fff;
}

.model-tag {
  font-size: 22px;
  font-weight: 400;
  color: #e3e3e3;
  padding: 0 8px;
}

.messages {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding-bottom: 24px;
  scroll-behavior: smooth;
  display: flex;
  flex-direction: column;
}

/* Welcome Screen */
.welcome {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 0 12%;
  max-width: 1000px;
  margin: 0 auto;
  width: 100%;
  text-align: center;
}

.welcome-title {
  font-size: 32px;
  font-weight: 500;
  background: linear-gradient(to right, #4285f4, #9b72cb, #d96570);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  margin-bottom: 8px;
  line-height: 1.4;
}

.welcome-sub {
  font-size: 32px;
  color: #444746;
  font-weight: 500;
  line-height: 1.4;
}

.main.new-chat-layout .messages {
  justify-content: center;
}

.main.new-chat-layout .composer-wrap {
  margin-bottom: 30vh;
}

.main.new-chat-layout .welcome {
  transform: translateY(42px);
}

/* Message Display */
.message {
  width: 100%;
  max-width: 840px;
  margin: 0 auto;
  padding: 30px 24px;
  display: flex;
  gap: 20px;
}

.role-user {
  flex-direction: row-reverse;
}

.message-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  flex-shrink: 0;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #3c4043;
}

.message-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.message-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.role-assistant .message-main {
  gap: 14px;
}

.role-user .message-main {
  align-items: flex-end;
}

.message-header {
  margin-bottom: 4px;
}

.message-model-name {
  font-size: 18px;
  font-weight: 550;
  letter-spacing: 0.1px;
  color: #fff;
}

.message-body {
  font-size: 16px;
  line-height: 1.72;
  color: #e3e3e3;
}

.role-user .message-avatar {
  background: #004a77;
  color: #c2e7ff;
  font-size: 14px;
  font-weight: 600;
}

.user-initial {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.role-user .message-body {
  background: #282829;
  padding: 12px 20px;
  border-radius: 24px;
  display: inline-block;
  max-width: 85%;
}

.message-actions {
  margin-top: 6px;
  display: flex;
  gap: 8px;
}

.btn-action {
  background: transparent;
  border: 0;
  padding: 8px;
  border-radius: 50%;
  color: #c4c7c5;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.btn-action:hover {
  background: rgba(255, 255, 255, 0.08);
  color: #fff;
}

/* Editing Style */
.message-edit-card {
  background: #1e1f20;
  border: 1px solid #444746;
  border-radius: 20px;
  padding: 16px;
  width: 100%;
  margin-top: 8px;
}

.message-edit-card textarea {
  width: 100%;
  min-height: 100px;
  background: transparent;
  border: 0;
  color: #fff;
  font-size: 16px;
  outline: none;
  resize: none;
  line-height: 1.6;
}

.message-edit-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 12px;
}

/* Thinking Style */
.thinking-text {
  font-size: 15px;
  color: #c4c7c5;
  font-style: italic;
  margin: 8px 0 10px;
  line-height: 1.8;
  animation: pulse 2s infinite ease-in-out;
}

@keyframes pulse {
  0%, 100% { opacity: 0.6; }
  50% { opacity: 1; }
}

.think-box {
  margin-bottom: 4px;
  padding: 10px 14px;
  border-radius: 14px;
  border: 1px solid rgba(149, 161, 183, 0.18);
  background: rgba(255, 255, 255, 0.03);
}

.think-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  background: transparent;
  border: 0;
  color: #c4c7c5;
  font-size: 14px;
  padding: 6px 0;
  cursor: pointer;
}

.think-toggle svg {
  transition: transform 0.2s;
}

.think-content {
  font-size: 14px;
  line-height: 1.72;
  color: #c4c7c5;
  font-style: italic;
  padding-left: 16px;
  border-left: 2px solid #444746;
  margin-top: 8px;
}

.typing {
  display: flex;
  gap: 5px;
  padding: 12px 0;
}

.typing span {
  width: 6px;
  height: 6px;
  background: #c4c7c5;
  border-radius: 50%;
  animation: typing 1s infinite ease-in-out;
}

.typing span:nth-child(2) { animation-delay: 0.2s; }
.typing span:nth-child(3) { animation-delay: 0.4s; }

@keyframes typing {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-5px); }
}

.messages-spacer {
  height: 32px;
}

/* Composer Card */
.composer-wrap {
  padding: 0 20px 24px;
  flex-shrink: 0;
}

.composer-card {
  max-width: 840px;
  margin: 0 auto;
  background: #1e1f20;
  border: 1px solid transparent;
  border-radius: 32px;
  padding: 12px 20px;
  display: flex;
  flex-direction: column;
  transition: background 0.2s;
}

.composer-card:focus-within {
  background: #28292a;
}

.composer-card textarea {
  width: 100%;
  min-height: 48px;
  max-height: 200px;
  background: transparent;
  border: 0;
  outline: none;
  padding: 12px 0;
  resize: none;
  font-size: 16px;
  line-height: 1.5;
  color: #fff;
}

.composer-bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 0;
}

.composer-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.composer-left select {
  background: transparent;
  border: 0;
  font-size: 14px;
  color: #d3d7de;
  cursor: pointer;
  outline: none;
  appearance: none;
  -webkit-appearance: none;
  padding: 8px 28px 8px 12px;
  min-width: 132px;
}

.model-switch-pill {
  position: relative;
  display: inline-flex;
  align-items: center;
  border-radius: 999px;
  border: 1px solid rgba(164, 174, 196, 0.3);
  background: linear-gradient(140deg, rgba(255, 255, 255, 0.06), rgba(255, 255, 255, 0.02));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08), 0 6px 16px rgba(0, 0, 0, 0.22);
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background 0.2s ease;
}

.model-switch-pill:hover,
.model-switch-pill:focus-within {
  border-color: rgba(196, 207, 230, 0.46);
  background: linear-gradient(140deg, rgba(255, 255, 255, 0.09), rgba(255, 255, 255, 0.03));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.14), 0 8px 18px rgba(0, 0, 0, 0.28);
}

.model-switch-pill svg {
  position: absolute;
  right: 10px;
  color: #b7bdc8;
  pointer-events: none;
}

.model-tag {
  display: inline-flex;
  align-items: center;
  min-height: 36px;
  border-radius: 999px;
  background: linear-gradient(140deg, rgba(255, 255, 255, 0.06), rgba(255, 255, 255, 0.015));
  border: 1px solid rgba(170, 182, 206, 0.23);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.09);
  backdrop-filter: blur(8px);
  padding: 0 14px;
  font-size: 15px;
  font-weight: 500;
  color: #e7ebf2;
}

.topbar-title-hidden {
  visibility: hidden;
  min-width: 136px;
}

.confirm-overlay {
  position: fixed;
  inset: 0;
  background: rgba(2, 6, 14, 0.68);
  backdrop-filter: blur(2px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 50;
  padding: 20px;
}

.confirm-modal {
  width: 100%;
  max-width: 420px;
  border-radius: 18px;
  border: 1px solid rgba(160, 172, 194, 0.25);
  background: linear-gradient(160deg, #1f222b, #171a22);
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.45), inset 0 1px 0 rgba(255, 255, 255, 0.08);
  padding: 20px;
}

.confirm-title {
  font-size: 18px;
  font-weight: 600;
  color: #f4f7ff;
  margin-bottom: 8px;
}

.confirm-text {
  font-size: 14px;
  line-height: 1.6;
  color: #c5cddd;
}

.confirm-actions {
  margin-top: 18px;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.confirm-danger {
  background: #eb5757;
  color: #fff;
}

.confirm-danger:hover:not(:disabled) {
  background: #f06262;
}

.btn-send {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: transparent;
  color: #fff;
  border: 0;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.2s;
}

.btn-send.stop {
  background: rgba(255, 255, 255, 0.16);
  color: #fff;
}

.btn-send.stop:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.24);
}

.btn-send:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.1);
}

.btn-send:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

/* Markdown Styles */
.markdown-body {
  word-break: break-word;
}

.markdown-body p { margin: 0 0 16px; }
.markdown-body p:last-child { margin-bottom: 0; }

.markdown-body pre {
  background: #131314;
  border: 1px solid #444746;
  border-radius: 12px;
  padding: 16px;
  overflow-x: auto;
  margin: 20px 0;
}

.markdown-body code {
  font-family: "Roboto Mono", monospace;
  font-size: 14px;
  background: rgba(255, 255, 255, 0.08);
  padding: 2px 6px;
  border-radius: 6px;
}

.markdown-body pre code {
  background: transparent;
  padding: 0;
}

.markdown-body table {
  width: 100%;
  border-collapse: collapse;
  margin: 20px 0;
}

.markdown-body th, .markdown-body td {
  border: 1px solid #444746;
  padding: 12px 16px;
  text-align: left;
}

.markdown-body blockquote {
  margin: 20px 0;
  padding-left: 20px;
  border-left: 4px solid #444746;
  color: #c4c7c5;
}

/* Login Overlay */
.login-overlay {
  position: fixed;
  inset: 0;
  background: var(--bg-main);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.login-card {
  width: 100%;
  max-width: 440px;
  padding: 40px;
  text-align: center;
}

.login-card h1 { font-size: 36px; margin-bottom: 32px; font-weight: 400; }
.login-card p { color: var(--text-secondary); margin-bottom: 32px; }
.login-card input {
  width: 100%;
  padding: 14px 20px;
  background: #1e1f20;
  border: 1px solid #444746;
  border-radius: 16px;
  margin-bottom: 20px;
  outline: none;
  color: #fff;
}

.btn-primary {
  background: #8ab4f8;
  color: #041e49;
  border: 0;
  font-weight: 500;
  border-radius: 24px;
  padding: 12px 24px;
}

.btn-ghost {
  background: transparent;
  border: 1px solid #444746;
  border-radius: 24px;
  padding: 8px 20px;
}

.btn-mini { font-size: 13px; padding: 6px 16px; }

@media (max-width: 768px) {
  .sidebar {
    position: fixed;
    transform: translateX(-100%);
    margin-left: 0;
    width: 280px;
  }
  .sidebar.closed {
    margin-left: 0;
    width: 280px;
    transform: translateX(-100%);
  }
  .sidebar.open {
    transform: translateX(0);
  }
  .mobile-only { display: flex; }
  .mobile-mask {
    display: block;
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.6);
    z-index: 5;
  }
  .welcome-title, .welcome-sub {
    font-size: 32px;
  }
}
</style>
