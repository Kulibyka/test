import { useState } from "react";

const mockFiles = [
  {
    shortName: "Срочно! Нужно согласовать бюджет",
    mainWords: "срочно, бюджет, согласование",
    approvers: "@asdasdas, @dasdasda, @chebureck",
    from: "manager1",
    to: "finance1",
    received: "25-11-2025",
    inputText: "Добрый день! Пожалуйста, согласуйте бюджет проекта на следующий квартал до конца дня. Без утверждения бюджета мы не сможем запускать закупки и новые задачи.",
    outputText: "Добрый день! Бюджет проекта на следующий квартал согласован. Все расходы проверены, закупки могут быть запущены согласно плану."
  },
  {
    shortName: "Запрос данных по клиенту",
    mainWords: "данные, клиент, запрос",
    from: "sales1",
    to: "crm_team",
    received: "24-11-2025",
    inputText: "Необходимо получить актуальные контактные данные по клиенту ООО «Альфа». Пожалуйста, пришлите информацию до конца дня, чтобы подготовить коммерческое предложение.",
    outputText: "Добрый день! Актуальные контактные данные по клиенту ООО «Альфа» подготовлены и отправлены на ваш e-mail. Можете использовать их для подготовки предложения."
  },
  {
    shortName: "Подтверждение встречи с партнёром",
    mainWords: "встреча, партнёр, подтверждение",
    from: "assistant1",
    to: "manager2",
    received: "23-11-2025",
    inputText: "Напоминаю, что завтра запланирована встреча с партнёром в 11:00. Пожалуйста, подтвердите участие и подготовьте материалы презентации.",
    outputText: "Встреча с партнёром подтверждена. Материалы презентации подготовлены, участие всех сотрудников обеспечено."
  },
  {
    shortName: "Срочно! Проблема с сервером",
    mainWords: "срочно, сервер, проблема",
    from: "it_support",
    to: "dev_team",
    received: "22-11-2025",
    inputText: "Срочно! На сервере production обнаружена ошибка, из-за которой недоступна часть сервисов. Требуется вмешательство команды разработки для устранения проблемы.",
    outputText: "Ошибка на сервере production устранена. Сервисы восстановлены. Дополнительно проведён анализ причин сбоя и подготовлены рекомендации по предотвращению подобных ситуаций."
  },
  {
    shortName: "Напоминание о сроке отчёта",
    mainWords: "срок, отчёт, напоминание",
    from: "project_lead",
    to: "team_members",
    received: "21-11-2025",
    inputText: "Добрый день! Напоминаю, что срок сдачи отчёта по проекту истекает завтра. Просьба завершить все задания и прислать финальные версии документов.",
    outputText: "Добрый день! Все задания завершены, финальные версии документов подготовлены и отправлены на проверку. Отчёт будет сдан в срок."
  }
];

const ITEMS_PER_PAGE = 5;

export default function ApprovementPage() {
  const [files, setFiles] = useState(mockFiles);
  const [visibleInputs, setVisibleInputs] = useState({});
  const [visibleOutputs, setVisibleOutputs] = useState({});
  const [searchTerm, setSearchTerm] = useState("");
  const [currentPage, setCurrentPage] = useState(1);

  const toggleInputs = (index) => {
    setVisibleInputs((prev) => ({
      ...prev,
      [index]: !prev[index],
    }));
  };

  const toggleOutput = (index) => {
    setVisibleOutputs((prev) => ({
      ...prev,
      [index]: !prev[index],
    }));
  };

  const filteredFiles = files.filter((file) => {
    const lowerSearch = searchTerm.toLowerCase();
    return (
      file.shortName.toLowerCase().includes(lowerSearch) ||
      file.inputText.toLowerCase().includes(lowerSearch) ||
      file.From.toLowerCase().includes(lowerSearch)
    );
  });

  const totalPages = Math.ceil(filteredFiles.length / ITEMS_PER_PAGE);
  const startIndex = (currentPage - 1) * ITEMS_PER_PAGE;
  const currentFiles = filteredFiles.slice(startIndex, startIndex + ITEMS_PER_PAGE);

  return (
    <div className="space-y-4 text-[16px] max-w-7xl mx-auto">
      <h2 className="text-2xl font-bold">Согласование</h2>

      <input
        type="text"
        placeholder="Поиск"
        value={searchTerm}
        onChange={(e) => {
          setSearchTerm(e.target.value);
          setCurrentPage(1);
        }}
        className="w-full border px-3 py-2 rounded"
      />

      {/* Прокручиваемый список */}
      <div className="h-[500px] overflow-y-auto border rounded p-2 bg-gray-50">
        <ul className="space-y-4">
          {currentFiles.map((file, index) => (
            <li key={startIndex + index} className="border p-4 rounded bg-white shadow-sm">
              <div className="flex justify-between items-center">
                <div>
                  <p className="font-semibold"> {file.shortName}</p>
                  <p className="text-sm text-black-500"> Необходимо согласование: {file.approvers}</p>
                  <p className="text-sm text-gray-500"> {file.mainWords}</p>
                  <p className="text-sm text-gray-500"> От: {file.from}</p>
                  <p className="text-sm text-gray-500"> Кому: {file.to}</p>
                  <p className="text-sm text-gray-500"> Получено: {file.received}</p>
                </div>
                <div className="flex gap-2 flex-wrap">
                  <button
                    onClick={() => toggleInputs(startIndex + index)}
                    className="bg-blue-600 text-white px-3 py-1 rounded hover:bg-blue-700"
                  >
                    Текст
                  </button>
                  <button
                    onClick={() => toggleOutput(startIndex + index)}
                    className="bg-blue-600 text-white px-3 py-1 rounded hover:bg-blue-700"
                  >
                    Ответ
                  </button>
                  <button
                    
                    className="bg-blue-600 text-white px-3 py-1 rounded hover:bg-blue-700"
                  >
                    Согласовать
                  </button>
                </div>
              </div>
              {visibleInputs[startIndex + index] && (
                <p className="text-base text-gray-700 mt-3"> Сообщение: {file.inputText}</p>
              )}
               {visibleOutputs[startIndex + index] && (
                <p className="text-base text-gray-700 mt-3"> Ответ: {file.outputText}</p>
              )}
            </li>
          ))}
          {filteredFiles.length === 0 && (
            <p className="text-gray-500">Сообщения не найдены.</p>
          )}
        </ul>
      </div>

      {/* Пагинация вне прокрутки */}
      {totalPages > 1 && (
        <div className="flex justify-center gap-4 items-center py-2 sticky bottom-0 bg-white border-t">
          <button
            disabled={currentPage === 1}
            onClick={() => setCurrentPage((p) => p - 1)}
            className="px-3 py-1 rounded bg-gray-200 hover:bg-gray-300 disabled:opacity-50"
          >
            Назад
          </button>
          <span>
            Страница {currentPage} из {totalPages}
          </span>
          <button
            disabled={currentPage === totalPages}
            onClick={() => setCurrentPage((p) => p + 1)}
            className="px-3 py-1 rounded bg-gray-200 hover:bg-gray-300 disabled:opacity-50"
          >
            Вперёд
          </button>
        </div>
      )}
    </div>
  );
}