import React, { useEffect, useState } from 'react';
import { SafeAreaView, View, Text, TextInput, Pressable, StyleSheet } from 'react-native';
import { initChatStore } from './src/chat/store';
import { setToken } from './src/chat/config';
import { ChatListScreen } from './src/screens/ChatListScreen';
import { ConversationScreen } from './src/screens/ConversationScreen';

// Демо-вход: вставьте токен и свой user id (в реале — SMS-логин как в вебе).
// Получить токен для теста можно из localStorage веб-приложения (ключ `token`).
function LoginGate({ onReady }: { onReady: (meId: number) => void }): React.ReactElement {
  const [tok, setTok] = useState('');
  const [meId, setMeId] = useState('');
  return (
    <View style={styles.login}>
      <Text style={styles.h1}>Manibandha · Чат</Text>
      <Text style={styles.label}>Токен доступа</Text>
      <TextInput style={styles.input} value={tok} onChangeText={setTok} autoCapitalize="none" placeholder="eyJ…" />
      <Text style={styles.label}>Ваш user id</Text>
      <TextInput style={styles.input} value={meId} onChangeText={setMeId} keyboardType="number-pad" placeholder="1" />
      <Pressable
        style={styles.btn}
        onPress={() => { if (tok && meId) { setToken(tok.trim()); onReady(Number(meId)); } }}
      >
        <Text style={styles.btnText}>Войти</Text>
      </Pressable>
    </View>
  );
}

export default function App(): React.ReactElement {
  const [meId, setMeId] = useState<number | null>(null);
  const [ready, setReady] = useState(false);
  const [openId, setOpenId] = useState<number | null>(null);

  useEffect(() => {
    if (meId == null) return;
    let cancelled = false;
    void (async () => {
      await initChatStore(meId);
      if (!cancelled) setReady(true);
    })();
    return () => { cancelled = true; };
  }, [meId]);

  return (
    <SafeAreaView style={styles.app}>
      {meId == null ? (
        <LoginGate onReady={setMeId} />
      ) : !ready ? (
        <Text style={styles.loading}>Подключение…</Text>
      ) : openId != null ? (
        <ConversationScreen chatId={openId} onBack={() => setOpenId(null)} />
      ) : (
        <ChatListScreen onOpen={setOpenId} />
      )}
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  app: { flex: 1, backgroundColor: '#fbf6ee' },
  loading: { padding: 20, color: '#8a7f6a' },
  login: { padding: 24, gap: 8 },
  h1: { fontSize: 24, fontWeight: '700', color: '#2b2a26', marginBottom: 12 },
  label: { fontSize: 13, color: '#8a7f6a', marginTop: 8 },
  input: { borderWidth: 1, borderColor: '#e3d6bf', borderRadius: 10, padding: 12, fontSize: 15, backgroundColor: '#fff' },
  btn: { marginTop: 16, backgroundColor: '#d99a4e', borderRadius: 10, padding: 14, alignItems: 'center' },
  btnText: { color: '#fff', fontSize: 16, fontWeight: '600' },
});
