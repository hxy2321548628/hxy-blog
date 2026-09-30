import { Route, Routes } from 'react-router-dom'
import PostListPage from './features/posts/PostListPage'

function App() {
  return (
    <Routes>
      <Route path="/" element={<PostListPage />} />
      <Route path="*" element={<PostListPage />} />
    </Routes>
  )
}

export default App
