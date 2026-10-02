import { useEffect, useRef, useState } from 'react'

export const DEFAULT_BACKGROUND_MUSIC_VOLUME = 0.05

type BackgroundAudio = Pick<
  HTMLAudioElement,
  'pause' | 'paused' | 'play' | 'volume'
>

type PageClickTarget = {
  addEventListener(type: 'click', listener: EventListener): void
  removeEventListener(type: 'click', listener: EventListener): void
}

export function listenForFirstPageClick(
  page: PageClickTarget,
  onFirstClick: () => void,
): () => void {
  let isListening = true

  const stopListening = () => {
    if (!isListening) {
      return
    }

    isListening = false
    page.removeEventListener('click', handleClick)
  }

  const handleClick: EventListener = () => {
    // 只消费首次点击，避免后续页面操作反复触发播放请求。
    stopListening()
    onFirstClick()
  }

  page.addEventListener('click', handleClick)
  return stopListening
}

export async function startBackgroundMusic(
  audio: BackgroundAudio,
): Promise<boolean> {
  // 浏览器没有音量 HTML 属性，必须先设置再播放，避免自动播放成功时短暂使用 100% 音量。
  audio.volume = DEFAULT_BACKGROUND_MUSIC_VOLUME

  try {
    await audio.play()
    return true
  } catch {
    // 首次访问常被浏览器拦截；这不是页面错误，保留按钮让访客主动开始播放。
    return false
  }
}

export async function toggleBackgroundMusic(
  audio: BackgroundAudio,
): Promise<boolean> {
  if (!audio.paused) {
    audio.pause()
    return false
  }

  try {
    await audio.play()
    return true
  } catch {
    return false
  }
}

function BackgroundMusic() {
  const audioRef = useRef<HTMLAudioElement>(null)
  const [isPlaying, setIsPlaying] = useState(false)

  useEffect(() => {
    const audio = audioRef.current
    if (!audio) {
      return
    }

    let isMounted = true
    let stopWaitingForClick: (() => void) | undefined
    void startBackgroundMusic(audio).then((started) => {
      if (isMounted) {
        setIsPlaying(started)

        if (!started) {
          // 自动播放被拦截后，首次页面点击提供浏览器要求的用户手势。
          stopWaitingForClick = listenForFirstPageClick(document, () => {
            void startBackgroundMusic(audio).then((startedAfterClick) => {
              if (isMounted) {
                setIsPlaying(startedAfterClick)
              }
            })
          })
        }
      }
    })

    return () => {
      isMounted = false
      stopWaitingForClick?.()
      // 组件只存在于前台，进入管理端时立即停止音乐。
      audio.pause()
    }
  }, [])

  const togglePlayback = async () => {
    const audio = audioRef.current
    if (!audio) {
      return
    }

    setIsPlaying(await toggleBackgroundMusic(audio))
  }

  const controlLabel = isPlaying ? '暂停背景音乐' : '播放背景音乐'

  return (
    <aside className="music-control" aria-label="背景音乐控制">
      <audio
        ref={audioRef}
        src="/audio/shade-of-light.mp3"
        loop
        preload="none"
        aria-hidden="true"
        onPlay={() => setIsPlaying(true)}
        onPause={() => setIsPlaying(false)}
      />
      <button
        type="button"
        className="music-control__button"
        aria-label={controlLabel}
        aria-pressed={isPlaying}
        title={controlLabel}
        onClick={() => void togglePlayback()}
      >
        <span className="music-control__icon" aria-hidden="true">
          {isPlaying ? 'Ⅱ' : '♪'}
        </span>
        <span>{isPlaying ? '暂停音乐' : '播放音乐'}</span>
      </button>
    </aside>
  )
}

export default BackgroundMusic
