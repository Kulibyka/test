import { BrowserRouter as Router, Routes, Route, NavLink } from "react-router-dom";
import { useState } from "react";
import HomePage from "./components/Homepage";
import Alerts from "./components/Alerts";
import ApprovementPage from "./components/ApprovementPage";

export default function App() {
  const [username, setUsername] = useState("");

  return (
    <Router>
      <div>
        {/* Навигация */}
        <div className="fixed top-0 left-0 w-full bg-blue-600 z-50 flex items-center justify-between px-4 py-3">
          <nav className="flex gap-4">
            <NavLink to="/" className="text-white">Главная</NavLink>
            <NavLink to="/alerts" className="text-white"> Уведомления</NavLink>
            <NavLink to="/approvement" className="text-white"> Согласование</NavLink>
          </nav>
          <div className="flex items-center gap-3">
      
          </div>
        </div>

        {/* Контент */}
        <div className="pt-20 px-4">
          <Routes>
            <Route path="/" element={<HomePage />} />
            <Route path="*" element={<HomePage />} />
            <Route path="/alerts" element={<Alerts />} />
            <Route path="/approvement" element={<ApprovementPage />} />
          </Routes>
        </div>
      </div>
    </Router>
  );
}
