import React, { useEffect, useState } from 'react';
import { View, Text, FlatList, TextInput, Pressable, StyleSheet, KeyboardAvoidingView, Platform } from 'react-native';
import { useChatState, getStore } from '../chat/store';
import type { ChatMessage } from '@manibandha/chat-core';

export function ConversationScreen({ chatId, onBack }: { chatId: number; onBack: () => void }): React.ReactElement {
  const state = useChatState();
  const [text, setText] = useState('');
  const meId = state.meId;
  const chat = state.chats.find((c) => c.id === chatId);

  useEffect(() => {
    void getStore().openChat(chatId);
    return () => getStore().closeChat();
  }, [chatId]);

  const send = () => {
    const t = text.trim();
    if (!t) return;
    setText('');
    void getStore().sendMessage(t);
  };

  const renderItem = ({ item }: { item: ChatMessage }) => {
    const mine = item.author_id === meId;
    return (
      <View style={[styles.bubbleRow, mine ? styles.rowRight : styles.rowLeft]}>
        <View style={[styles.bubble, mine ? styles.bubbleMine : styles.bubbleOther]}>
          {!mine && chat?.type === 'group' && item.author_name ? (
            <Text style={styles.author}>{item.author_name}</Text>
          ) : null}
          <Text style={[styles.body, mine && styles.bodyMine]}>{item.body}</Text>
          <Text style={[styles.time, mine && styles.timeMine]}>
            {item.status === 'pending' ? '⏳' : item.status === 'failed' ? '⚠️' : '✓'}
          </Text>
        </View>
      </View>
    );
  };

  return (
    <KeyboardAvoidingView style={styles.container} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <View style={styles.header}>
        <Pressable onPress={onBack}><Text style={styles.back}>‹ Назад</Text></Pressable>
        <Text style={styles.headerTitle} numberOfLines={1}>{chat?.title || 'Чат'}</Text>
      </View>
      <FlatList
        style={styles.list}
        data={state.messages}
        keyExtractor={(m) => m.client_uuid}
        renderItem={renderItem}
        contentContainerStyle={{ padding: 10 }}
      />
      <View style={styles.composer}>
        <TextInput
          style={styles.input}
          value={text}
          onChangeText={(t) => { setText(t); getStore().sendTyping(); }}
          placeholder="Сообщение…"
          multiline
          maxLength={1000}
        />
        <Pressable style={styles.sendBtn} onPress={send}><Text style={styles.sendText}>➤</Text></Pressable>
      </View>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#fbf6ee' },
  header: { flexDirection: 'row', alignItems: 'center', gap: 12, padding: 12, borderBottomWidth: 1, borderBottomColor: '#efe6d6' },
  back: { color: '#c8792e', fontSize: 16 },
  headerTitle: { fontSize: 17, fontWeight: '600', color: '#2b2a26', flex: 1 },
  list: { flex: 1 },
  bubbleRow: { marginVertical: 3, flexDirection: 'row' },
  rowRight: { justifyContent: 'flex-end' },
  rowLeft: { justifyContent: 'flex-start' },
  bubble: { maxWidth: '80%', borderRadius: 16, paddingHorizontal: 12, paddingVertical: 8 },
  bubbleMine: { backgroundColor: '#d99a4e' },
  bubbleOther: { backgroundColor: '#fff', borderWidth: 1, borderColor: '#efe6d6' },
  author: { fontSize: 12, fontWeight: '600', color: '#6f7a5a', marginBottom: 2 },
  body: { fontSize: 15, color: '#2b2a26' },
  bodyMine: { color: '#fff' },
  time: { fontSize: 10, color: '#8a7f6a', textAlign: 'right', marginTop: 2 },
  timeMine: { color: 'rgba(255,255,255,0.8)' },
  composer: { flexDirection: 'row', alignItems: 'flex-end', gap: 8, padding: 8, borderTopWidth: 1, borderTopColor: '#efe6d6' },
  input: { flex: 1, maxHeight: 120, borderWidth: 1, borderColor: '#e3d6bf', borderRadius: 18, paddingHorizontal: 14, paddingVertical: 8, fontSize: 15, backgroundColor: '#fdfaf3' },
  sendBtn: { width: 40, height: 40, borderRadius: 20, backgroundColor: '#d99a4e', alignItems: 'center', justifyContent: 'center' },
  sendText: { color: '#fff', fontSize: 18 },
});
