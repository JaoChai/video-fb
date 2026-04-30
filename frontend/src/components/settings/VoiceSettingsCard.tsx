import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '../ui/card';

const TTS_VOICES = [
  'Achird', 'Charon', 'Sadaltager', 'Sulafat', 'Rasalgethi',
  'Puck', 'Iapetus', 'Schedar', 'Gacrux', 'Algieba',
  'Zephyr', 'Kore', 'Fenrir', 'Leda', 'Orus', 'Aoede',
  'Achernar', 'Alnilam', 'Despina', 'Erinome',
];

interface VoiceSettingsCardProps {
  value: string;
  onChange: (key: string, value: string) => void;
}

export function VoiceSettingsCard({ value, onChange }: VoiceSettingsCardProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Voice Settings</CardTitle>
        <CardDescription>Select the TTS voice for video narration</CardDescription>
      </CardHeader>
      <CardContent>
        <label className="block text-xs text-muted-foreground uppercase tracking-wide mb-1.5">
          TTS Voice (Gemini)
        </label>
        <select
          value={value}
          onChange={e => onChange('elevenlabs_voice', e.target.value)}
          className="h-10 w-full max-w-xs rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 cursor-pointer"
        >
          {TTS_VOICES.map(v => (
            <option key={v} value={v}>{v}</option>
          ))}
        </select>
      </CardContent>
    </Card>
  );
}
