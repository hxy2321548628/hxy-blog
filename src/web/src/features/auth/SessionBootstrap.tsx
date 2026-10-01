import { useEffect } from 'react'
import { useAppDispatch } from '../../app/hooks'
import { restoreSession } from './authSlice'

function SessionBootstrap() {
  const dispatch = useAppDispatch()

  useEffect(() => {
    void dispatch(restoreSession())
  }, [dispatch])

  return null
}

export default SessionBootstrap
