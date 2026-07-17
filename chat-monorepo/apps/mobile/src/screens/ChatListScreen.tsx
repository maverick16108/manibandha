import React from 'react';
import { View, Text, FlatList, Pressable, StyleSheet } from 'react-native';
import { useChatState } from '../chat/store';
import type { ChatListItem } from '@manibandha/chat-core';

function initials(name: string): string {
  return (name || '?').trim()[0]?.toUpperCase() || '?';
}

export function ChatListScreen({ onOpen }: { onOpen: (id: number) => void }): React.ReactElement {
  const state = useChatState();

  const renderItem = ({ item }: { item: ChatListItem }) => (
    <Pressable style={styles.row} onPress={() => onOpen(item.id)}>
      <View style={[styles.avatar, item.type === 'group' ? styles.avatarGroup : styles.avatarDirect]}>
        <Text style={styles.avatarText}>{initials(item.title)}</Text>
      </View>
      <View style={styles.rowBody}>
        <Text style={styles.title} numberOfLines={1}>{item.title}</Text>
        <Text style={styles.preview} numberOfLines={1}>
          {item.last ? (item.last.deleted ? 'сообщение удалено' : (item.last.body || '')) : 'Нет сообщений'}
        </Text>
      </View>
      {item.unread > 0 && (
        <View style={styles.badge}><Text style={styles.badgeText}>{item.unread}</Text></View>
      )}
    </Pressable>
  );

  return (
    <View style={styles.container}>
      {!state.ready ? (
        <Text style={styles.loading}>Загрузка…</Text>
      ) : (
        <FlatList data={state.chats} keyExtractor={(c) => String(c.id)} renderItem={renderItem} />
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, backgroundColor: '#fbf6ee' },
  loading: { padding: 16, color: '#8a7f6a' },
  row: { flexDirection: 'row', alignItems: 'center', gap: 12, paddingHorizontal: 14, paddingVertical: 10, borderBottomWidth: 1, borderBottomColor: '#efe6d6' },
  avatar: { width: 46, height: 46, borderRadius: 23, alignItems: 'center', justifyContent: 'center' },
  avatarDirect: { backgroundColor: '#d99a4e' },
  avatarGroup: { backgroundColor: '#6f7a5a' },
  avatarText: { color: '#fff', fontWeight: '600', fontSize: 18 },
  rowBody: { flex: 1, minWidth: 0 },
  title: { fontSize: 16, fontWeight: '600', color: '#2b2a26' },
  preview: { fontSize: 14, color: '#8a7f6a' },
  badge: { minWidth: 22, height: 22, borderRadius: 11, backgroundColor: '#d99a4e', alignItems: 'center', justifyContent: 'center', paddingHorizontal: 6 },
  badgeText: { color: '#fff', fontSize: 12, fontWeight: '600' },
});
