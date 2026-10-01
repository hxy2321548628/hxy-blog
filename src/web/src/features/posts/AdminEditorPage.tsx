import { type FormEvent, useState } from 'react'
import { Link, Navigate, useNavigate, useParams } from 'react-router-dom'
import type { ApiError } from '../../app/httpClient'
import MarkdownContent from '../../components/MarkdownContent'
import AdminHeader from '../auth/AdminHeader'
import {
  useCreateDraftMutation,
  useGetAdminPostQuery,
  usePublishPostMutation,
  useUpdateDraftMutation,
  type AdminPostDetail,
  type DraftInput,
} from './postApi'

function mutationMessage(error: unknown) {
  const candidate = error as Partial<ApiError>
  return typeof candidate.message === 'string'
    ? candidate.message
    : '操作失败，请稍后重试。'
}

interface EditorFormProps {
  initial?: AdminPostDetail
}

function EditorForm({ initial }: EditorFormProps) {
  const navigate = useNavigate()
  const [createDraft, createState] = useCreateDraftMutation()
  const [updateDraft, updateState] = useUpdateDraftMutation()
  const [publishPost, publishState] = usePublishPostMutation()
  const [slug, setSlug] = useState(initial?.slug ?? '')
  const [title, setTitle] = useState(initial?.title ?? '')
  const [contentMarkdown, setContentMarkdown] = useState(
    initial?.contentMarkdown ?? '',
  )
  const [message, setMessage] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)
  const isNew = !initial
  const published = initial?.status === 'published'

  const draft = (): DraftInput => ({
    slug: slug.trim(),
    title: title.trim(),
    contentMarkdown,
  })

  const save = async (): Promise<AdminPostDetail> => {
    const input = draft()
    if (!input.slug || !input.title) {
      throw new Error('请填写 slug 和标题。')
    }
    if (isNew) {
      const created = await createDraft(input).unwrap()
      navigate(`/admin/posts/${created.id}`, { replace: true })
      return created
    }
    return updateDraft({ id: initial.id, draft: input }).unwrap()
  }

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setFormError(null)
    setMessage(null)
    try {
      await save()
      setMessage('草稿已保存。')
    } catch (error: unknown) {
      setFormError(
        error instanceof Error ? error.message : mutationMessage(error),
      )
    }
  }

  const publish = async () => {
    setFormError(null)
    setMessage(null)
    try {
      const saved = await save()
      const result = await publishPost(saved.id).unwrap()
      navigate(`/posts/${encodeURIComponent(result.slug)}`)
    } catch (error: unknown) {
      setFormError(
        error instanceof Error ? error.message : mutationMessage(error),
      )
    }
  }

  const pending =
    createState.isLoading || updateState.isLoading || publishState.isLoading

  return (
    <form className="editor" onSubmit={(event) => void submit(event)}>
      <section className="editor-form" aria-labelledby="editor-fields-title">
        <div className="editor-section-heading">
          <div>
            <p className="eyebrow">DRAFT</p>
            <h2 id="editor-fields-title">文章内容</h2>
          </div>
          {published && (
            <span className="status-badge status-badge--published">已发布</span>
          )}
        </div>

        <label htmlFor="post-title">标题</label>
        <input
          id="post-title"
          maxLength={200}
          required
          disabled={published}
          value={title}
          onChange={(event) => setTitle(event.target.value)}
        />

        <label htmlFor="post-slug">Slug</label>
        <input
          id="post-slug"
          maxLength={200}
          pattern="[a-z0-9]+(?:-[a-z0-9]+)*"
          placeholder="my-first-post"
          required
          disabled={published}
          value={slug}
          onChange={(event) => setSlug(event.target.value)}
        />
        <p className="field-help">仅使用小写字母、数字和单个连字符。</p>

        <label htmlFor="post-content">Markdown 正文</label>
        <textarea
          id="post-content"
          disabled={published}
          value={contentMarkdown}
          onChange={(event) => setContentMarkdown(event.target.value)}
        />

        {formError && (
          <p className="auth-form__error" role="alert">
            {formError}
          </p>
        )}
        {message && (
          <p className="editor-message" role="status">
            {message}
          </p>
        )}

        <div className="editor-actions">
          {published ? (
            <Link
              className="button-primary"
              to={`/posts/${encodeURIComponent(slug)}`}
            >
              查看已发布文章
            </Link>
          ) : (
            <>
              <button
                className="button-secondary"
                type="submit"
                disabled={pending}
              >
                {pending ? '正在保存…' : '保存草稿'}
              </button>
              <button
                className="button-primary"
                type="button"
                disabled={pending}
                onClick={() => void publish()}
              >
                {publishState.isLoading ? '正在发布…' : '保存并发布'}
              </button>
            </>
          )}
        </div>
      </section>

      <section className="editor-preview" aria-labelledby="preview-title">
        <div className="editor-section-heading">
          <div>
            <p className="eyebrow">PREVIEW</p>
            <h2 id="preview-title">实时预览</h2>
          </div>
        </div>
        <div className="article-content">
          {contentMarkdown.trim() ? (
            <MarkdownContent>{contentMarkdown}</MarkdownContent>
          ) : (
            <p className="editor-preview__empty">
              开始输入 Markdown 后，这里会显示预览。
            </p>
          )}
        </div>
      </section>
    </form>
  )
}

function AdminEditorPage() {
  const { id } = useParams()
  const isNew = id === 'new'
  const numericID = Number(id)
  const validID = Number.isSafeInteger(numericID) && numericID > 0
  const { data, error, isLoading } = useGetAdminPostQuery(numericID, {
    skip: isNew || !validID,
  })

  if (!isNew && !validID) {
    return <Navigate to="/admin" replace />
  }

  return (
    <main className="admin-shell admin-shell--editor">
      <AdminHeader title={isNew ? '新建草稿' : '编辑文章'} backTo="/admin" />
      {isLoading && (
        <p className="request-state" role="status">
          正在读取草稿…
        </p>
      )}
      {error && (
        <p className="request-state request-state--error" role="alert">
          草稿不存在或暂时无法读取。
        </p>
      )}
      {isNew && <EditorForm key="new" />}
      {data && <EditorForm key={data.id} initial={data} />}
    </main>
  )
}

export default AdminEditorPage
