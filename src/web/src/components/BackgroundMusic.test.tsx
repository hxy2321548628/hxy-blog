import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import BackgroundMusic, {
  DEFAULT_BACKGROUND_MUSIC_VOLUME,
  startBackgroundMusic,
  toggleBackgroundMusic,
} from './BackgroundMusic'

function createAudioStub(paused = true) {
  return {
    pause: vi.fn(),
    paused,
    play: vi.fn().mockResolvedValue(undefined),
    volume: 1,
  }
}

describe('BackgroundMusic', () => {
  it('输出循环播放但不预加载完整音频的前台控件', () => {
    const html = renderToStaticMarkup(<BackgroundMusic />)

    expect(html).toContain('src="/audio/shade-of-light.mp3"')
    expect(html).toContain('loop=""')
    expect(html).toContain('preload="none"')
    expect(html).toContain('aria-label="播放背景音乐"')
  })

  it('以 10% 音量尝试自动播放', async () => {
    const audio = createAudioStub()

    await expect(startBackgroundMusic(audio)).resolves.toBe(true)
    expect(audio.volume).toBe(DEFAULT_BACKGROUND_MUSIC_VOLUME)
    expect(audio.play).toHaveBeenCalledOnce()
  })

  it('自动播放被浏览器阻止时保持暂停状态', async () => {
    const audio = createAudioStub()
    audio.play.mockRejectedValueOnce(new Error('autoplay blocked'))

    await expect(startBackgroundMusic(audio)).resolves.toBe(false)
  })

  it('在播放和暂停之间切换', async () => {
    const playingAudio = createAudioStub(false)
    const pausedAudio = createAudioStub(true)

    await expect(toggleBackgroundMusic(playingAudio)).resolves.toBe(false)
    expect(playingAudio.pause).toHaveBeenCalledOnce()
    await expect(toggleBackgroundMusic(pausedAudio)).resolves.toBe(true)
    expect(pausedAudio.play).toHaveBeenCalledOnce()
  })
})
