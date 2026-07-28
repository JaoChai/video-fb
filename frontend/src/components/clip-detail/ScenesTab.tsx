import { useState } from 'react';
import type { Scene } from '../../api';
import { ImageOff } from 'lucide-react';

function SceneImage({ url }: { url: string | null }) {
  const [failed, setFailed] = useState(false);

  if (!url || failed) {
    return (
      <div className="w-[104px] shrink-0 aspect-[9/16] rounded-md bg-muted flex items-center justify-center">
        <ImageOff className="size-4 text-muted-foreground opacity-50" />
      </div>
    );
  }
  return (
    <img
      src={url}
      onError={() => setFailed(true)}
      className="w-[104px] shrink-0 aspect-[9/16] object-cover rounded-md bg-black"
    />
  );
}

export function ScenesTab({ scenes }: { scenes: Scene[] }) {
  if (scenes.length === 0) {
    return <p className="text-sm text-muted-foreground">คลิปนี้ยังไม่มีฉาก (ผลิตไม่ถึงขั้นแตกฉาก)</p>;
  }

  return (
    <div className="space-y-3">
      {scenes.map(s => (
        <div key={s.id} className="flex gap-3 rounded-lg border p-3">
          <SceneImage url={s.image_9_16_url} />
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 flex-wrap text-xs text-muted-foreground mb-1">
              <span className="font-medium text-foreground">ฉาก {s.scene_number}</span>
              {s.scene_type && <span>{s.scene_type}</span>}
              {s.beat && <span>· {s.beat}</span>}
              {s.layout && <span>· {s.layout}</span>}
              <span>· {s.duration_seconds.toFixed(1)} วิ</span>
            </div>
            {s.on_screen_text && (
              <p className="text-sm font-medium leading-snug mb-1 break-words">{s.on_screen_text}</p>
            )}
            {s.voice_text && (
              <p className="text-xs text-muted-foreground leading-relaxed whitespace-pre-wrap break-words">
                {s.voice_text}
              </p>
            )}
          </div>
        </div>
      ))}
    </div>
  );
}
