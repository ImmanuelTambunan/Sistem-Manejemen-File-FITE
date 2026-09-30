<?php

namespace App\Http\Controllers;

use App\Models\Category;
use App\Models\Department;
use App\Models\Document;
use Illuminate\Http\Request;

class HomeController extends Controller
{
    public function index(Request $request)
    {
        $categories = Category::all();
        $departments = Department::all();

        // Query dokumen dengan relasi
        $query = Document::with(['department', 'category', 'uploader.role']);

        // Filter berdasarkan kategori tab jika dipilih
        if ($request->filled('category')) {
            $query->whereHas('category', function ($q) use ($request) {
                $q->where('slug', $request->category);
            });
        }

        // Ambil data terbaru (limit 9 dokumen untuk landing page)
        $documents = $query->latest()->paginate(9)->withQueryString();

        return view('home', compact('categories', 'departments', 'documents'));
    }
}