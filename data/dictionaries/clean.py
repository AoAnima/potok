input_file = './english_words.txt'
output_file = './english_words.txt'

# Чтение файла
with open(input_file, 'r', encoding='utf-8') as f:
    text = f.read()

# Замена запятых на перенос строки и разбиение на слова
words = text.replace(',', '\n').split()

# Удаление дубликатов с сохранением порядка
seen = set()
unique_words = []
for word in words:
    if word not in seen:
        seen.add(word)
        unique_words.append(word)

# Запись результата в файл
with open(output_file, 'w', encoding='utf-8') as f:
    f.write('\n'.join(unique_words))

print(f"Очищенные слова сохранены в {output_file}")
print(f"Всего уникальных слов: {len(unique_words)}")