#! /usr/bin/env python3

import os

def get_factor(line):
    parts = line.split('|')
    return parts[len(parts)-2]

for file in os.listdir('.'):
    if file[-2:] != 'md':
        continue

    with open(file, 'r') as fp:
        min = 1000
        lines = list(fp)
        data = lines[2:]

        for i, line in enumerate(data):
            parts = line.split('|')

            times = parts[2].strip()
            time = float(times.split('±')[0])

            if time < min:
                min = time

        for i, line in enumerate(data):
            parts = line.split('|')

            times = parts[2].strip()
            time = float(times.split('±')[0])

            factor = round(time / min, 2)

            if 'Change' in lines[0]:
                parts.pop(-3) # drop Change column

            parts[len(parts)-2] = f' {factor:.2f} '
            line = '|'.join(parts)
            data[i] = line

        data.sort(key=get_factor)

    with open(file, 'w') as fp:
        if 'Change' in lines[0]:
            lines[0] = "| Command | Mean Wall Time [s] | Factor |\n"

        fp.write(''.join(lines[0:2]))
        fp.write(''.join(data))

print('ok')