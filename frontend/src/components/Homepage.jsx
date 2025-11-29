import { useNavigate } from "react-router-dom";

export default function Homepage() {
  const navigate = useNavigate();

  return (
    <div className="h-screen flex flex-col justify-center items-center bg-gradient-to-br from-blue-50 to-white px-4">
      <h1 className="text-5xl font-extrabold text-blue-800 text-center mb-8">
        Админка
      </h1>

      <div className="max-w-4xl w-full bg-white p-6 rounded-2xl shadow-lg space-y-6 text-lg text-gray-800">
        <p>
          Добро пожаловать!
        </p>
      </div>
    </div>
  );
}
