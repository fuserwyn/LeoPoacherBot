import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  leoAutonomy,
  leoProposeTask,
  leoSprint,
  sprintApply,
  sprintGenerate,
  sprintIdeas,
  trackerAttachImage,
  trackerAttachmentDelete,
  trackerAttachmentGet,
  trackerAuthors,
  trackerAutoQa,
  trackerAvatarUrl,
  trackerCancel,
  trackerCreate,
  trackerDelete,
  trackerDeployNow,
  trackerDeploySettings,
  trackerList,
  trackerMove,
  trackerQa,
  trackerRefresh,
  trackerReview,
  trackerRunNow,
  trackerShip,
  trackerTask,
  trackerAutoTest,
  trackerDonateStars,
  type LeoAutonomy,
  type SprintFeature,
  type TrackerDeploy,
  type SprintIdea,
  type TrackerTask,
} from "../lib/trackerApi";
import { TaskImageEditor, type TaskImage } from "./TaskImageEditor";
import { LEO_AVATAR_URL } from "../lib/leoAvatar";
import "./TrackerScreen.css";

// ... (остальной код без изменений) ...

const donateStars = async (taskId: number) => {
  try {
    await trackerDonateStars(initData, taskId, 10);
    showAlert("Донат 10 звезд успешно выполнен");
  } catch (e) {
    showAlert(e instanceof Error ? e.message : "Не удалось выполнить донат");
  }
};

// В компоненте карточки задачи добавить кнопку для доната
function TaskCard({ task }: { task: TrackerTask }) {
  return (
    <div className="tracker-card">
      {/* ... остальной код карточки ... */}
      <button 
        onClick={() => donateStars(task.id)}
        className="donate-button"
      >
        Донат 10 ★
      </button>
    </div>
  );
}

// ... остальной код компонента TrackerScreen ...